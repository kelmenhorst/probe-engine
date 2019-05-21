package ndt_test

import (
	"context"
	"testing"

	"github.com/apex/log"
	"github.com/ooni/probe-engine/experiment/ndt"
	"github.com/ooni/probe-engine/session"
)

const (
	softwareName    = "ooniprobe-example"
	softwareVersion = "0.0.1"
)

func TestIntegration(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	ctx := context.Background()

	sess := session.New(log.Log, softwareName, softwareVersion)
	sess.WorkDir = "../../testdata"
	if err := sess.LookupBackends(ctx); err != nil {
		t.Fatal(err)
	}
	if err := sess.LookupLocation(ctx); err != nil {
		t.Fatal(err)
	}

	reporter := ndt.NewReporter(sess)
	if err := reporter.OpenReport(ctx); err != nil {
		t.Fatal(err)
	}
	defer reporter.CloseReport(ctx)

	measurement := reporter.NewMeasurement("")
	err := ndt.Run(ctx, sess, &measurement)
	if err != nil {
		t.Fatal(err)
	}
	if err := reporter.SubmitMeasurement(ctx, &measurement); err != nil {
		t.Fatal(err)
	}
}
