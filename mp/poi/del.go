// @description wechat 是腾讯微信公众平台 api 的 golang 语言封装
// @link        https://gopkg.in/chanxuehong/wechat.v1 for the canonical source repository
// @license     https://github.com/chanxuehong/wechat/blob/v1/LICENSE
// @authors     chanxuehong(chanxuehong@gmail.com)

package poi

import (
	"gopkg.in/chanxuehong/wechat.v1/mp"
)

// 删除门店.
func (clt *Client) PoiDelete(poiId int64) (err error) {
	var request = struct {
		PoiId int64 `json:"poi_id"`
	}{
		PoiId: poiId,
	}

	var result mp.Error

	incompleteURL := "https://api.weixin.qq.com/cgi-bin/poi/delpoi?access_token="
	if err = ((*mp.Client)(clt)).PostJSON(incompleteURL, &request, &result); err != nil {
		return
	}

	if result.ErrCode != mp.ErrCodeOK {
		err = &result
		return
	}
	return
}
