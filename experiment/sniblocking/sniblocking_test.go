package sniblocking

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apex/log"
	"github.com/ooni/probe-engine/internal/mockable"
	"github.com/ooni/probe-engine/model"
	"github.com/ooni/probe-engine/netx/errorx"
)

const (
	softwareName    = "ooniprobe-example"
	softwareVersion = "0.0.1"
)

func TestTestKeysClassify(t *testing.T) {
	asStringPtr := func(s string) *string {
		return &s
	}
	t.Run("with tk.Target.Failure == nil", func(t *testing.T) {
		tk := new(TestKeys)
		if tk.classify() != classSuccessGotServerHello {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == connection_refused", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr(errorx.FailureConnectionRefused)
		if tk.classify() != classAnomalyTestHelperUnreachable {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == dns_nxdomain_error", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr(errorx.FailureDNSNXDOMAINError)
		if tk.classify() != classAnomalyTestHelperUnreachable {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == connection_reset", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr(errorx.FailureConnectionReset)
		if tk.classify() != classInterferenceReset {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == eof_error", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr(errorx.FailureEOFError)
		if tk.classify() != classInterferenceClosed {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == ssl_invalid_hostname", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr(errorx.FailureSSLInvalidHostname)
		if tk.classify() != classSuccessGotServerHello {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == ssl_unknown_authority", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr(errorx.FailureSSLUnknownAuthority)
		if tk.classify() != classInterferenceUnknownAuthority {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == ssl_invalid_certificate", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr(errorx.FailureSSLInvalidCertificate)
		if tk.classify() != classInterferenceInvalidCertificate {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == generic_timeout_error #1", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr(errorx.FailureGenericTimeoutError)
		if tk.classify() != classAnomalyTimeout {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == generic_timeout_error #2", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr(errorx.FailureGenericTimeoutError)
		tk.Control.Failure = asStringPtr(errorx.FailureGenericTimeoutError)
		if tk.classify() != classAnomalyTestHelperUnreachable {
			t.Fatal("unexpected result")
		}
	})
	t.Run("with tk.Target.Failure == unknown_failure", func(t *testing.T) {
		tk := new(TestKeys)
		tk.Target.Failure = asStringPtr("unknown_failure")
		if tk.classify() != classAnomalyUnexpectedFailure {
			t.Fatal("unexpected result")
		}
	})
}

func TestNewExperimentMeasurer(t *testing.T) {
	measurer := NewExperimentMeasurer(Config{})
	if measurer.ExperimentName() != "sni_blocking" {
		t.Fatal("unexpected name")
	}
	if measurer.ExperimentVersion() != "0.1.0" {
		t.Fatal("unexpected version")
	}
}

func TestMeasurerMeasureNoControlSNI(t *testing.T) {
	measurer := NewExperimentMeasurer(Config{})
	err := measurer.Run(
		context.Background(),
		newsession(),
		new(model.Measurement),
		model.NewPrinterCallbacks(log.Log),
	)
	if err.Error() != "Experiment requires ControlSNI" {
		t.Fatal("not the error we expected")
	}
}

func TestMeasurerMeasureNoMeasurementInput(t *testing.T) {
	measurer := NewExperimentMeasurer(Config{
		ControlSNI: "example.com",
	})
	err := measurer.Run(
		context.Background(),
		newsession(),
		new(model.Measurement),
		model.NewPrinterCallbacks(log.Log),
	)
	if err.Error() != "Experiment requires measurement.Input" {
		t.Fatal("not the error we expected")
	}
}

func TestMeasurerMeasureWithInvalidInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel the context
	measurer := NewExperimentMeasurer(Config{
		ControlSNI: "example.com",
	})
	measurement := &model.Measurement{
		Input: "\t",
	}
	err := measurer.Run(
		ctx,
		newsession(),
		measurement,
		model.NewPrinterCallbacks(log.Log),
	)
	if err == nil {
		t.Fatal("expected an error here")
	}
}

func TestMeasurerMeasureWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel the context
	measurer := NewExperimentMeasurer(Config{
		ControlSNI: "example.com",
	})
	measurement := &model.Measurement{
		Input: "kernel.org",
	}
	err := measurer.Run(
		ctx,
		newsession(),
		measurement,
		model.NewPrinterCallbacks(log.Log),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMeasureoneCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel the context
	result := new(Measurer).measureone(
		ctx,
		&mockable.Session{MockableLogger: log.Log},
		time.Now(),
		"kernel.org",
		"example.com:443",
	)
	if result.Agent != "" {
		t.Fatal("not the expected Agent")
	}
	if result.BootstrapTime != 0.0 {
		t.Fatal("not the expected BootstrapTime")
	}
	if result.DNSCache != nil {
		t.Fatal("not the expected DNSCache")
	}
	if result.FailedOperation == nil || *result.FailedOperation != errorx.TopLevelOperation {
		t.Fatal("not the expected FailedOperation")
	}
	if result.Failure == nil || *result.Failure != errorx.FailureInterrupted {
		t.Fatal("not the expected failure")
	}
	if result.NetworkEvents != nil {
		t.Fatal("not the expected NetworkEvents")
	}
	if result.Queries != nil {
		t.Fatal("not the expected Queries")
	}
	if result.Requests != nil {
		t.Fatal("not the expected Requests")
	}
	if result.SOCKSProxy != "" {
		t.Fatal("not the expected SOCKSProxy")
	}
	if result.TCPConnect != nil {
		t.Fatal("not the expected TCPConnect")
	}
	if result.TLSHandshakes != nil {
		t.Fatal("not the expected TLSHandshakes")
	}
	if result.Tunnel != "" {
		t.Fatal("not the expected Tunnel")
	}
	if result.SNI != "kernel.org" {
		t.Fatal("unexpected SNI")
	}
	if result.THAddress != "example.com:443" {
		t.Fatal("unexpected THAddress")
	}
}

func TestMeasureoneWithPreMeasurementFailure(t *testing.T) {
	result := new(Measurer).measureone(
		context.Background(),
		&mockable.Session{MockableLogger: log.Log},
		time.Now(),
		"kernel.org",
		"example.com:443\t\t\t", // cause URL parse error
	)
	if result.Agent != "redirect" {
		t.Fatal("not the expected Agent")
	}
	if result.BootstrapTime != 0.0 {
		t.Fatal("not the expected BootstrapTime")
	}
	if result.DNSCache != nil {
		t.Fatal("not the expected DNSCache")
	}
	if result.FailedOperation == nil || *result.FailedOperation != "top_level" {
		t.Fatal("not the expected FailedOperation")
	}
	if result.Failure == nil || !strings.Contains(*result.Failure, "invalid target URL") {
		t.Fatal("not the expected failure")
	}
	if result.NetworkEvents != nil {
		t.Fatal("not the expected NetworkEvents")
	}
	if result.Queries != nil {
		t.Fatal("not the expected Queries")
	}
	if result.Requests != nil {
		t.Fatal("not the expected Requests")
	}
	if result.SOCKSProxy != "" {
		t.Fatal("not the expected SOCKSProxy")
	}
	if result.TCPConnect != nil {
		t.Fatal("not the expected TCPConnect")
	}
	if result.TLSHandshakes != nil {
		t.Fatal("not the expected TLSHandshakes")
	}
	if result.Tunnel != "" {
		t.Fatal("not the expected Tunnel")
	}
	if result.SNI != "kernel.org" {
		t.Fatal("unexpected SNI")
	}
	if result.THAddress != "example.com:443\t\t\t" {
		t.Fatal("unexpected THAddress")
	}
}

func TestMeasureoneSuccess(t *testing.T) {
	result := new(Measurer).measureone(
		context.Background(),
		&mockable.Session{MockableLogger: log.Log},
		time.Now(),
		"kernel.org",
		"example.com:443",
	)
	if result.Agent != "redirect" {
		t.Fatal("not the expected Agent")
	}
	if result.BootstrapTime != 0.0 {
		t.Fatal("not the expected BootstrapTime")
	}
	if result.DNSCache != nil {
		t.Fatal("not the expected DNSCache")
	}
	if result.FailedOperation == nil || *result.FailedOperation != errorx.TLSHandshakeOperation {
		t.Fatal("not the expected FailedOperation")
	}
	if result.Failure == nil || *result.Failure != errorx.FailureSSLInvalidHostname {
		t.Fatal("unexpected failure")
	}
	if len(result.NetworkEvents) < 1 {
		t.Fatal("not the expected NetworkEvents")
	}
	if len(result.Queries) < 1 {
		t.Fatal("not the expected Queries")
	}
	if result.Requests != nil {
		t.Fatal("not the expected Requests")
	}
	if result.SOCKSProxy != "" {
		t.Fatal("not the expected SOCKSProxy")
	}
	if len(result.TCPConnect) < 1 {
		t.Fatal("not the expected TCPConnect")
	}
	if len(result.TLSHandshakes) < 1 {
		t.Fatal("not the expected TLSHandshakes")
	}
	if result.Tunnel != "" {
		t.Fatal("not the expected Tunnel")
	}
	if result.SNI != "kernel.org" {
		t.Fatal("unexpected SNI")
	}
	if result.THAddress != "example.com:443" {
		t.Fatal("unexpected THAddress")
	}
}

func TestMeasureonewithcacheWorks(t *testing.T) {
	measurer := &Measurer{cache: make(map[string]Subresult)}
	output := make(chan Subresult, 2)
	for i := 0; i < 2; i++ {
		measurer.measureonewithcache(
			context.Background(),
			output,
			&mockable.Session{MockableLogger: log.Log},
			time.Now(),
			"kernel.org",
			"example.com:443",
		)
	}
	for _, expected := range []bool{false, true} {
		result := <-output
		if result.Cached != expected {
			t.Fatal("unexpected cached")
		}
		if *result.Failure != errorx.FailureSSLInvalidHostname {
			t.Fatal("unexpected failure")
		}
		if result.SNI != "kernel.org" {
			t.Fatal("unexpected SNI")
		}
	}
}

func TestProcessallPanicsIfInvalidSNI(t *testing.T) {
	defer func() {
		panicdata := recover()
		if panicdata == nil {
			t.Fatal("expected to see panic here")
		}
		if panicdata.(string) != "unexpected smk.SNI" {
			t.Fatal("not the panic we expected")
		}
	}()
	outputs := make(chan Subresult, 1)
	measurement := &model.Measurement{
		Input: "kernel.org",
	}
	go func() {
		outputs <- Subresult{
			SNI: "antani.io",
		}
	}()
	processall(
		outputs,
		measurement,
		model.NewPrinterCallbacks(log.Log),
		[]string{"kernel.org", "example.com"},
		newsession(),
		"example.com",
	)
}

func TestMaybeURLToSNI(t *testing.T) {
	t.Run("for invalid URL", func(t *testing.T) {
		parsed, err := maybeURLToSNI("\t")
		if err == nil {
			t.Fatal("expected an error here")
		}
		if parsed != "" {
			t.Fatal("expected empty parsed here")
		}
	})
	t.Run("for domain name", func(t *testing.T) {
		parsed, err := maybeURLToSNI("kernel.org")
		if err != nil {
			t.Fatal(err)
		}
		if parsed != "kernel.org" {
			t.Fatal("expected different domain here")
		}
	})
	t.Run("for valid URL", func(t *testing.T) {
		parsed, err := maybeURLToSNI("https://kernel.org/robots.txt")
		if err != nil {
			t.Fatal(err)
		}
		if parsed != "kernel.org" {
			t.Fatal("expected different domain here")
		}
	})
}

func newsession() model.ExperimentSession {
	return &mockable.Session{MockableLogger: log.Log}
}
