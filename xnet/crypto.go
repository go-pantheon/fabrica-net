// Package xnet provides the network layer for the fabrica-net.
package xnet

import (
	"slices"

	"github.com/go-pantheon/fabrica-util/security/aes"
)

var _ Cryptor = (*cryptor)(nil)

type cryptor struct {
	encrypt bool
	key     []byte
	aes     *aes.Cipher
}

// NewCryptor creates a new cryptor.
func NewCryptor(key []byte) (Cryptor, error) {
	aes, err := aes.NewAESCipher(key)
	if err != nil {
		return nil, err
	}

	return &cryptor{
		encrypt: true,
		key:     key,
		aes:     aes,
	}, nil
}

func (c *cryptor) IsCrypto() bool {
	return c.encrypt
}

func (c *cryptor) Encrypt(data Pack) (Pack, error) {
	if !c.encrypt {
		return slices.Clone(data), nil
	}

	return c.aes.Encrypt(data)
}

func (c *cryptor) Decrypt(data Pack) (Pack, error) {
	if !c.encrypt {
		return slices.Clone(data), nil
	}

	return c.aes.Decrypt(data)
}

func (c *cryptor) Key() []byte {
	return c.key
}

// NewUnCryptor creates a new uncryptor.
func NewUnCryptor() Cryptor {
	return &cryptor{
		encrypt: false,
		key:     []byte{},
		aes:     nil,
	}
}
