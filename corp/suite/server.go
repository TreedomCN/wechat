// @description wechat 是腾讯微信公众平台 api 的 golang 语言封装
// @link        https://gopkg.in/chanxuehong/wechat.v1 for the canonical source repository
// @license     https://github.com/chanxuehong/wechat/blob/v1/LICENSE
// @authors     chanxuehong(chanxuehong@gmail.com)

package suite

import (
	"bytes"
	"errors"
	"sync"
)

type Server interface {
	SuiteToken() string // 套件的Token
	SuiteId() string    // 套件Id, 用于约束消息的 ToUserName, 如果为空表示不约束

	CurrentAESKey() [32]byte                // 获取当前有效的 AES 加密 Key
	LastAESKey() (key [32]byte, valid bool) // 获取上一个有效的 AES 加密 Key

	MessageHandler() MessageHandler // 获取 MessageHandler
}

var _ Server = (*DefaultServer)(nil)

type DefaultServer struct {
	suiteId    string
	suiteToken string

	rwmutex           sync.RWMutex
	currentAESKey     [32]byte // 当前的 AES Key
	lastAESKey        [32]byte // 最后一个 AES Key
	isLastAESKeyValid bool     // lastAESKey 是否有效, 如果 lastAESKey 是 zero 则无效

	messageHandler MessageHandler
}

// NewDefaultServer 创建一个新的 DefaultServer.
func NewDefaultServer(suiteId, suiteToken string, AESKey []byte, handler MessageHandler) (srv *DefaultServer) {
	if len(AESKey) != 32 {
		panic("the length of AESKey must equal to 32")
	}
	if handler == nil {
		panic("nil MessageHandler")
	}

	srv = &DefaultServer{
		suiteId:        suiteId,
		suiteToken:     suiteToken,
		messageHandler: handler,
	}
	copy(srv.currentAESKey[:], AESKey)
	return
}

func (srv *DefaultServer) SuiteId() string {
	return srv.suiteId
}
func (srv *DefaultServer) SuiteToken() string {
	return srv.suiteToken
}
func (srv *DefaultServer) MessageHandler() MessageHandler {
	return srv.messageHandler
}
func (srv *DefaultServer) CurrentAESKey() (key [32]byte) {
	srv.rwmutex.RLock()
	key = srv.currentAESKey
	srv.rwmutex.RUnlock()
	return
}
func (srv *DefaultServer) LastAESKey() (key [32]byte, valid bool) {
	srv.rwmutex.RLock()
	key = srv.lastAESKey
	valid = srv.isLastAESKeyValid
	srv.rwmutex.RUnlock()
	return
}

func (srv *DefaultServer) UpdateAESKey(aesKey []byte) (err error) {
	if len(aesKey) != 32 {
		return errors.New("the length of aesKey must equal to 32")
	}

	srv.rwmutex.Lock()
	defer srv.rwmutex.Unlock()

	if bytes.Equal(aesKey, srv.currentAESKey[:]) {
		return
	}

	srv.isLastAESKeyValid = true
	srv.lastAESKey = srv.currentAESKey
	copy(srv.currentAESKey[:], aesKey)
	return
}
