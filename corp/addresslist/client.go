// @description wechat 是腾讯微信公众平台 api 的 golang 语言封装
// @link        https://gopkg.in/chanxuehong/wechat.v1 for the canonical source repository
// @license     https://github.com/chanxuehong/wechat/blob/v1/LICENSE
// @authors     chanxuehong(chanxuehong@gmail.com)

package addresslist

import (
	"net/http"

	"gopkg.in/chanxuehong/wechat.v1/corp"
)

type Client corp.Client

func NewClient(srv corp.AccessTokenServer, clt *http.Client) *Client {
	return (*Client)(corp.NewClient(srv, clt))
}
