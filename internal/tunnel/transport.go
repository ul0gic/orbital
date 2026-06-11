package tunnel

// Transport exposes a local HTTP address to the public internet.
// Implementations own a single tunnel for the session lifetime.
type Transport interface {
	Start(localAddr string) (publicURL string, err error)
	Stop() error
}
