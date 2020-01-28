// Package resolverlookup discovers the resolver's IP
package resolverlookup

import (
	"context"
	"errors"
	"net"
)

// HostLookupper is an interface that looks up the name of a host.
type HostLookupper interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

// All returns all resolver IPs
func All(ctx context.Context, resolver HostLookupper) (ips []string, err error) {
	if resolver == nil {
		resolver = &net.Resolver{}
	}
	ips, err = resolver.LookupHost(ctx, "whoami.akamai.net")
	return
}

// First returns the first resolver IP
func First(ctx context.Context, resolver HostLookupper) (ip string, err error) {
	var ips []string
	ips, err = All(ctx, resolver)
	if err == nil && len(ips) < 1 {
		err = errors.New("No IP address returned")
		return
	}
	ip = ips[0]
	return
}
