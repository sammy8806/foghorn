//go:build (darwin && !cgo) || (!darwin && !linux && !windows)

package keyring

type unsupportedStore struct{}

func newStore(string) Store { return unsupportedStore{} }
func supported() bool       { return false }
func backendName() string   { return "" }

func (unsupportedStore) Get(string) ([]byte, error) { return nil, ErrUnsupported }
func (unsupportedStore) Set(string, []byte) error   { return ErrUnsupported }
func (unsupportedStore) Delete(string) error        { return ErrUnsupported }
