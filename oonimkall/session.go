package oonimkall

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	engine "github.com/ooni/probe-engine"
	"github.com/ooni/probe-engine/atomicx"
	"github.com/ooni/probe-engine/internal/runtimex"
	"github.com/ooni/probe-engine/model"
	"github.com/ooni/probe-engine/probeservices"
)

// The following two variables contain metrics pertaining to the number
// of Sessions and Contexts that are currently being used.
var (
	ActiveSessions = atomicx.NewInt64()
	ActiveContexts = atomicx.NewInt64()
)

// Logger is the logger used by a Session. You should implement a class
// compatible with this interface in Java/ObjC and then save a reference
// to this instance in the SessionConfig object. All log messages that
// the Session will generate will be routed to this Logger.
type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
}

// SessionConfig contains configuration for a Session. You should
// fill all the mandatory fields and could also optionally fill some of
// the optional fields. Then pass this struct to NewSession.
type SessionConfig struct {
	// AssetsDir is the mandatory directory where to store assets
	// required by a Session, e.g. MaxMind DB files.
	AssetsDir string

	// Logger is the optional logger that will receive all the
	// log messages generated by a Session. If this field is nil
	// then the session will not emit any log message.
	Logger Logger

	// ProbeServicesURL allows you to optionally force the
	// usage of an alternative probe service instance. This setting
	// should only be used for implementing integration tests.
	ProbeServicesURL string

	// SoftwareName is the mandatory name of the application
	// that will be using the new Session.
	SoftwareName string

	// SoftwareVersion is the mandatory version of the application
	// that will be using the new Session.
	SoftwareVersion string

	// StateDir is the mandatory directory where to store state
	// information required by a Session.
	StateDir string

	// TempDir is the mandatory directory where the Session shall
	// store temporary files. Among other tasks, Session.Close will
	// remove any temporary file created within this Session.
	TempDir string

	// Verbose is optional. If there is a non-null Logger and this
	// field is true, then the Logger will also receive Debug messages,
	// otherwise it will not receive such messages.
	Verbose bool
}

// Session contains shared state for running experiments and/or other
// OONI related task (e.g. geolocation). Note that the Session isn't
// mean to be a long living object. The workflow is to create a Session,
// do the operations you need to do with it now, then make sure it is
// not referenced by other variables, so the Go GC can finalize it.
//
// Future directions
//
// We will eventually rewrite the code for running new experiments such
// that a Task will be created from a Session, such that experiments
// could share the same Session and save geolookups, etc. For now, we
// are in the suboptimal situations where Tasks create, use, and close
// their own session, thus running more lookups than needed.
type Session struct {
	cl        []context.CancelFunc
	mtx       sync.Mutex
	submitter *probeservices.Submitter
	sessp     *engine.Session
}

// NewSession creates a new session. You should use a session for running
// a set of operations in a relatively short time frame. You SHOULD NOT create
// a single session and keep it all alive for the whole app lifecyle, since
// the Session code is not specifically designed for this use case.
func NewSession(config *SessionConfig) (*Session, error) {
	kvstore, err := engine.NewFileSystemKVStore(config.StateDir)
	if err != nil {
		return nil, err
	}
	var availableps []model.Service
	if config.ProbeServicesURL != "" {
		availableps = append(availableps, model.Service{
			Address: config.ProbeServicesURL,
			Type:    "https",
		})
	}
	engineConfig := engine.SessionConfig{
		AssetsDir:              config.AssetsDir,
		AvailableProbeServices: availableps,
		KVStore:                kvstore,
		Logger:                 newLogger(config.Logger, config.Verbose),
		SoftwareName:           config.SoftwareName,
		SoftwareVersion:        config.SoftwareVersion,
		TempDir:                config.TempDir,
	}
	sessp, err := engine.NewSession(engineConfig)
	if err != nil {
		return nil, err
	}
	sess := &Session{sessp: sessp}
	runtime.SetFinalizer(sess, sessionFinalizer)
	ActiveSessions.Add(1)
	return sess, nil
}

// sessionFinalizer finalizes a Session. While in general in Go code using a
// finalizer is probably unclean, it seems that using a finalizer when binding
// with Java/ObjC code is actually useful to simplify the apps.
func sessionFinalizer(sess *Session) {
	for _, fn := range sess.cl {
		fn()
	}
	if sess.submitter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		sess.submitter.Close(ctx) // ignore return value
	}
	sess.sessp.Close() // ignore return value
	ActiveSessions.Add(-1)
}

// Context is the context of an operation. You use this context
// to cancel a long running operation by calling Cancel(). Because
// you create a Context from a Session and because the Session is
// keeping track of the Context instances it owns, you do don't
// need to call the Cancel method when you're done.
type Context struct {
	cancel context.CancelFunc
	ctx    context.Context
}

// Cancel cancels pending operations using this context.
func (ctx *Context) Cancel() {
	ctx.cancel()
}

// NewContext creates an new interruptible Context.
func (sess *Session) NewContext() *Context {
	return sess.NewContextWithTimeout(-1)
}

// NewContextWithTimeout creates an new interruptible Context that will automatically
// cancel itself after the given timeout. Setting a zero or negative timeout implies
// there is no actual timeout configured for the Context.
func (sess *Session) NewContextWithTimeout(timeout int64) *Context {
	sess.mtx.Lock()
	defer sess.mtx.Unlock()
	ctx, origcancel := newContext(timeout)
	ActiveContexts.Add(1)
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			ActiveContexts.Add(-1)
			origcancel()
		})
	}
	sess.cl = append(sess.cl, cancel)
	return &Context{cancel: cancel, ctx: ctx}
}

// GeolocateResults contains the GeolocateTask results.
type GeolocateResults struct {
	// ASN is the autonomous system number.
	ASN string

	// Country is the country code.
	Country string

	// IP is the IP address.
	IP string

	// Org is the commercial name of the ASN.
	Org string
}

// Geolocate performs a geolocate operation and returns the results. This method
// is (in Java terminology) synchronized with the session instance.
func (sess *Session) Geolocate(ctx *Context) (*GeolocateResults, error) {
	sess.mtx.Lock()
	defer sess.mtx.Unlock()
	info, err := sess.sessp.LookupLocationContext(ctx.ctx)
	if err != nil {
		return nil, err
	}
	return &GeolocateResults{
		ASN:     fmt.Sprintf("AS%d", info.ASN),
		Country: info.CountryCode,
		IP:      info.ProbeIP,
		Org:     info.NetworkName,
	}, nil
}

// SubmitMeasurementResults contains the results of a single measurement submission
// to the OONI backends using the OONI collector API.
type SubmitMeasurementResults struct {
	UpdatedMeasurement string
	UpdatedReportID    string
}

// Submit submits the given measurement and returns the results. This method is (in
// Java terminology) synchronized with the Session instance.
func (sess *Session) Submit(ctx *Context, measurement string) (*SubmitMeasurementResults, error) {
	sess.mtx.Lock()
	defer sess.mtx.Unlock()
	if sess.submitter == nil {
		psc, err := sess.sessp.NewProbeServicesClient(ctx.ctx)
		if err != nil {
			return nil, err
		}
		sess.submitter = probeservices.NewSubmitter(psc)
	}
	var mm model.Measurement
	if err := json.Unmarshal([]byte(measurement), &mm); err != nil {
		return nil, err
	}
	if err := sess.submitter.Submit(ctx.ctx, &mm); err != nil {
		return nil, err
	}
	data, err := json.Marshal(mm)
	runtimex.PanicOnError(err, "json.Marshal should not fail here")
	return &SubmitMeasurementResults{
		UpdatedMeasurement: string(data),
		UpdatedReportID:    mm.ReportID,
	}, nil
}
