// @description wechat 是腾讯微信公众平台 api 的 golang 语言封装
// @link        https://gopkg.in/chanxuehong/wechat.v1 for the canonical source repository
// @license     https://github.com/chanxuehong/wechat/blob/v1/LICENSE
// @authors     chanxuehong(chanxuehong@gmail.com)

// +build wechatdebug

package suite

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/chanxuehong/util/security"

	"gopkg.in/chanxuehong/wechat.v1/corp"
	"gopkg.in/chanxuehong/wechat.v1/internal/util"
)

// 微信服务器请求 http body
type RequestHttpBody struct {
	XMLName struct{} `xml:"xml" json:"-"`

	SuiteId      string `xml:"ToUserName"`
	EncryptedMsg string `xml:"Encrypt"`
}

// ServeHTTP 处理 http 消息请求
//  NOTE: 调用者保证所有参数有效
func ServeHTTP(w http.ResponseWriter, r *http.Request, queryValues url.Values, srv Server, errHandler corp.ErrorHandler) {
	corp.LogInfoln("[WECHAT_DEBUG] request uri:", r.RequestURI)
	corp.LogInfoln("[WECHAT_DEBUG] request remote-addr:", r.RemoteAddr)
	corp.LogInfoln("[WECHAT_DEBUG] request user-agent:", r.UserAgent())

	switch r.Method {
	case "POST": // 消息处理
		msgSignature1 := queryValues.Get("msg_signature")
		if msgSignature1 == "" {
			errHandler.ServeError(w, r, errors.New("msg_signature is empty"))
			return
		}

		timestampStr := queryValues.Get("timestamp")
		if timestampStr == "" {
			errHandler.ServeError(w, r, errors.New("timestamp is empty"))
			return
		}

		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			err = errors.New("can not parse timestamp to int64: " + timestampStr)
			errHandler.ServeError(w, r, err)
			return
		}

		nonce := queryValues.Get("nonce")
		if nonce == "" {
			errHandler.ServeError(w, r, errors.New("nonce is empty"))
			return
		}

		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errHandler.ServeError(w, r, err)
			return
		}
		corp.LogInfoln("[WECHAT_DEBUG] request msg http body:\r\n", string(reqBody))

		// 解析 RequestHttpBody
		var requestHttpBody RequestHttpBody
		if err := xml.Unmarshal(reqBody, &requestHttpBody); err != nil {
			errHandler.ServeError(w, r, err)
			return
		}

		haveSuiteId := requestHttpBody.SuiteId
		wantSuiteId := srv.SuiteId()
		if wantSuiteId != "" && !security.SecureCompareString(haveSuiteId, wantSuiteId) {
			err = fmt.Errorf("the RequestHttpBody's ToUserName mismatch, have: %s, want: %s", haveSuiteId, wantSuiteId)
			errHandler.ServeError(w, r, err)
			return
		}

		suiteToken := srv.SuiteToken()

		// 验证签名
		msgSignature2 := util.MsgSign(suiteToken, timestampStr, nonce, requestHttpBody.EncryptedMsg)
		if !security.SecureCompareString(msgSignature1, msgSignature2) {
			err = fmt.Errorf("check msg_signature failed, input: %s, local: %s", msgSignature1, msgSignature2)
			errHandler.ServeError(w, r, err)
			return
		}

		// 解密
		encryptedMsgBytes, err := base64.StdEncoding.DecodeString(requestHttpBody.EncryptedMsg)
		if err != nil {
			errHandler.ServeError(w, r, err)
			return
		}

		aesKey := srv.CurrentAESKey()
		random, rawMsgXML, aesSuiteId, err := util.AESDecryptMsg(encryptedMsgBytes, aesKey)
		if err != nil {
			// 尝试用上一次的 AESKey 来解密
			lastAESKey, isLastAESKeyValid := srv.LastAESKey()
			if !isLastAESKeyValid {
				errHandler.ServeError(w, r, err)
				return
			}

			aesKey = lastAESKey // NOTE

			random, rawMsgXML, aesSuiteId, err = util.AESDecryptMsg(encryptedMsgBytes, aesKey)
			if err != nil {
				errHandler.ServeError(w, r, err)
				return
			}
		}
		if haveSuiteId != string(aesSuiteId) {
			err = fmt.Errorf("the RequestHttpBody's ToUserName(==%s) mismatch the SuiteId with aes encrypt(==%s)", haveSuiteId, aesSuiteId)
			errHandler.ServeError(w, r, err)
			return
		}

		corp.LogInfoln("[WECHAT_DEBUG] request msg raw xml:\r\n", string(rawMsgXML))

		// 解密成功, 解析 MixedMessage
		var mixedMsg MixedMessage
		if err = xml.Unmarshal(rawMsgXML, &mixedMsg); err != nil {
			errHandler.ServeError(w, r, err)
			return
		}

		// 安全考虑再次验证
		if haveSuiteId != mixedMsg.SuiteId {
			err = fmt.Errorf("the RequestHttpBody's ToUserName(==%s) mismatch the MixedMessage's SuiteId(==%s)", haveSuiteId, mixedMsg.SuiteId)
			errHandler.ServeError(w, r, err)
			return
		}

		// 成功, 交给 MessageHandler
		req := &Request{
			SuiteToken: suiteToken,

			HttpRequest: r,
			QueryValues: queryValues,

			MsgSignature: msgSignature1,
			Timestamp:    timestamp,
			Nonce:        nonce,

			RawMsgXML: rawMsgXML,
			MixedMsg:  &mixedMsg,

			AESKey:  aesKey,
			Random:  random,
			SuiteId: haveSuiteId,
		}
		srv.MessageHandler().ServeMessage(w, req)

	case "GET": // 首次验证
		msgSignature1 := queryValues.Get("msg_signature")
		if msgSignature1 == "" {
			errHandler.ServeError(w, r, errors.New("msg_signature is empty"))
			return
		}

		timestamp := queryValues.Get("timestamp")
		if timestamp == "" {
			errHandler.ServeError(w, r, errors.New("timestamp is empty"))
			return
		}

		nonce := queryValues.Get("nonce")
		if nonce == "" {
			errHandler.ServeError(w, r, errors.New("nonce is empty"))
			return
		}

		encryptedMsg := queryValues.Get("echostr")
		if encryptedMsg == "" {
			errHandler.ServeError(w, r, errors.New("echostr is empty"))
			return
		}

		msgSignature2 := util.MsgSign(srv.SuiteToken(), timestamp, nonce, encryptedMsg)
		if !security.SecureCompareString(msgSignature1, msgSignature2) {
			err := fmt.Errorf("check msg_signature failed, input: %s, local: %s", msgSignature1, msgSignature2)
			errHandler.ServeError(w, r, err)
			return
		}

		// 解密
		encryptedMsgBytes, err := base64.StdEncoding.DecodeString(encryptedMsg)
		if err != nil {
			errHandler.ServeError(w, r, err)
			return
		}

		aesKey := srv.CurrentAESKey()
		_, echostr, aesAppId, err := util.AESDecryptMsg(encryptedMsgBytes, aesKey)
		if err != nil {
			errHandler.ServeError(w, r, err)
			return
		}

		wantAppId := srv.SuiteId()
		if wantAppId != "" && !security.SecureCompare(aesAppId, []byte(wantAppId)) {
			err = fmt.Errorf("AppId with aes encrypt mismatch, have: %s, want: %s", aesAppId, wantAppId)
			errHandler.ServeError(w, r, err)
			return
		}

		w.Write(echostr)
	}
}
