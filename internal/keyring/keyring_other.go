//go:build !darwin

package keyring

type unsupportedStore struct{}

func newStore(string) Store { return unsupportedStore{} }
func supported() bool       { return false }

func (unsupportedStore) Get(string) ([]byte, error) { return nil, ErrUnsupported }
func (unsupportedStore) Set(string, []byte) error   { return ErrUnsupported }
func (unsupportedStore) Delete(string) error        { return ErrUnsupported }
