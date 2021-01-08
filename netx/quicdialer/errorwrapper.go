package quicdialer

import (
	"context"
	"crypto/tls"

	"github.com/lucas-clemente/quic-go"
	"github.com/ooni/probe-engine/legacy/netx/dialid"
	"github.com/ooni/probe-engine/netx/errorx"
)

// QUICErrorWrapperDialer is a dialer that performs quic err wrapping
type QUICErrorWrapperDialer struct {
	Dialer QUICContextDialer
}

// DialContext implements QUICContextDialer.DialContext
func (d QUICErrorWrapperDialer) DialContext(ctx context.Context, network string, addr string, host string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlySession, error) {
	dialID := dialid.ContextDialID(ctx)
	sess, err := d.Dialer.DialContext(ctx, network, addr, host, tlsCfg, cfg)
	err = errorx.SafeErrWrapperBuilder{
		// ConnID does not make any sense if we've failed and the error
		// does not make any sense (and is nil) if we succeded.
		DialID:    dialID,
		Error:     err,
		Operation: errorx.QUICHandshakeOperation,
	}.MaybeBuild()
	if err != nil {
		return nil, err
	}
	return sess, nil
}
