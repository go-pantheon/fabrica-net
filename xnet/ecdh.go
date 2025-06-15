package xnet

import (
	"github.com/go-pantheon/fabrica-util/security/ecdh"
)

var _ ECDHable = (*ecdhInfo)(nil)

type ecdhInfo struct {
	svrPri    [32]byte
	svrPub    [32]byte
	cliPub    [32]byte
	sharedKey []byte
}

func NewUnECDH() ECDHable {
	return &ecdhInfo{
		svrPri:    [32]byte{},
		svrPub:    [32]byte{},
		cliPub:    [32]byte{},
		sharedKey: []byte{},
	}
}

func NewECDHInfo(cliPub [32]byte) (ECDHable, error) {
	svrPri, svrPub, err := ecdh.GenKeyPair()
	if err != nil {
		return nil, err
	}

	sharedKey, err := ecdh.ComputeSharedKey(svrPri, cliPub)
	if err != nil {
		return nil, err
	}

	return &ecdhInfo{
		svrPri:    svrPri,
		svrPub:    svrPub,
		cliPub:    cliPub,
		sharedKey: sharedKey,
	}, nil
}

func (e *ecdhInfo) ClientPublicKey() []byte {
	return ecdh.KeyToBytes(&e.cliPub)
}

func (e *ecdhInfo) ServerPublicKey() []byte {
	return ecdh.KeyToBytes(&e.svrPub)
}

func (e *ecdhInfo) SharedKey() []byte {
	return e.sharedKey
}
