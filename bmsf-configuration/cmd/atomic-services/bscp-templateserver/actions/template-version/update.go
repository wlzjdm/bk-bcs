/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.,
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under,
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package templateversion

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/viper"

	"bk-bscp/cmd/middle-services/bscp-authserver/modules/auth"
	"bk-bscp/internal/audit"
	"bk-bscp/internal/authorization"
	"bk-bscp/internal/database"
	pbauthserver "bk-bscp/internal/protocol/authserver"
	pbcommon "bk-bscp/internal/protocol/common"
	pbdatamanager "bk-bscp/internal/protocol/datamanager"
	pb "bk-bscp/internal/protocol/templateserver"
	"bk-bscp/pkg/bkrepo"
	"bk-bscp/pkg/common"
	"bk-bscp/pkg/kit"
	"bk-bscp/pkg/logger"
)

// UpdateAction update a template version object
type UpdateAction struct {
	kit        kit.Kit
	viper      *viper.Viper
	authSvrCli pbauthserver.AuthClient
	dataMgrCli pbdatamanager.DataManagerClient

	req  *pb.UpdateConfigTemplateVersionReq
	resp *pb.UpdateConfigTemplateVersionResp
}

// NewUpdateAction creates new UpdateAction
func NewUpdateAction(kit kit.Kit, viper *viper.Viper,
	authSvrCli pbauthserver.AuthClient, dataMgrCli pbdatamanager.DataManagerClient,
	req *pb.UpdateConfigTemplateVersionReq, resp *pb.UpdateConfigTemplateVersionResp) *UpdateAction {

	action := &UpdateAction{
		kit:        kit,
		viper:      viper,
		authSvrCli: authSvrCli,
		dataMgrCli: dataMgrCli,
		req:        req,
		resp:       resp,
	}

	action.resp.Result = true
	action.resp.Code = pbcommon.ErrCode_E_OK
	action.resp.Message = "OK"

	return action
}

// Err setup error code message in response and return the error.
func (act *UpdateAction) Err(errCode pbcommon.ErrCode, errMsg string) error {
	if errCode != pbcommon.ErrCode_E_OK {
		act.resp.Result = false
	}
	act.resp.Code = errCode
	act.resp.Message = errMsg
	return errors.New(errMsg)
}

// Input handles the input messages.
func (act *UpdateAction) Input() error {
	if err := act.verify(); err != nil {
		return act.Err(pbcommon.ErrCode_E_TPL_PARAMS_INVALID, err.Error())
	}
	return nil
}

// Authorize checks the action authorization.
func (act *UpdateAction) Authorize() error {
	if errCode, errMsg := act.authorize(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}

// Output handles the output messages.
func (act *UpdateAction) Output() error {
	// do nothing.
	return nil
}

func (act *UpdateAction) verify() error {
	var err error

	if err = common.ValidateString("biz_id", act.req.BizId,
		database.BSCPNOTEMPTY, database.BSCPIDLENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateString("template_id", act.req.TemplateId,
		database.BSCPNOTEMPTY, database.BSCPIDLENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateString("version_id", act.req.VersionId,
		database.BSCPNOTEMPTY, database.BSCPIDLENLIMIT); err != nil {
		return err
	}
	act.req.ContentId = strings.ToUpper(act.req.ContentId)
	if err = common.ValidateString("content_id", act.req.ContentId,
		database.BSCPCONTENTIDLENLIMIT, database.BSCPCONTENTIDLENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateUint32("content_size", act.req.ContentSize, 0, math.MaxUint32); err != nil {
		return err
	}
	if err = common.ValidateString("memo", act.req.Memo, 0, database.BSCPLONGSTRLENLIMIT); err != nil {
		return err
	}

	return nil
}

func (act *UpdateAction) authorize() (pbcommon.ErrCode, string) {
	// check authorize resource at first, it may be deleted.
	if errCode, errMsg := act.queryConfigTemplate(); errCode != pbcommon.ErrCode_E_OK {
		return errCode, errMsg
	}

	// check resource authorization.
	isAuthorized, err := authorization.Authorize(act.kit, act.req.TemplateId, auth.LocalAuthAction,
		act.authSvrCli, act.viper.GetDuration("authserver.callTimeout"))
	if err != nil {
		return pbcommon.ErrCode_E_TPL_SYSTEM_UNKNOWN, fmt.Sprintf("authorize failed, %+v", err)
	}

	if !isAuthorized {
		return pbcommon.ErrCode_E_NOT_AUTHORIZED, "not authorized"
	}
	return pbcommon.ErrCode_E_OK, ""
}

func (act *UpdateAction) queryConfigTemplate() (pbcommon.ErrCode, string) {
	r := &pbdatamanager.QueryConfigTemplateReq{
		Seq:        act.kit.Rid,
		BizId:      act.req.BizId,
		TemplateId: act.req.TemplateId,
	}

	ctx, cancel := context.WithTimeout(act.kit.Ctx, act.viper.GetDuration("datamanager.callTimeout"))
	defer cancel()

	logger.V(4).Infof("UpdateConfigTemplateVersion[%s]| request to DataManager, %+v", r.Seq, r)

	resp, err := act.dataMgrCli.QueryConfigTemplate(ctx, r)
	if err != nil {
		return pbcommon.ErrCode_E_TPL_SYSTEM_UNKNOWN,
			fmt.Sprintf("request to DataManager QueryConfigTemplate, %+v", err)
	}
	return resp.Code, resp.Message
}

func (act *UpdateAction) validateContent() (pbcommon.ErrCode, string) {
	if !act.req.ValidateContent {
		return pbcommon.ErrCode_E_OK, "OK"
	}

	contentURL, err := bkrepo.GenContentURL(fmt.Sprintf("http://%s", act.viper.GetString("bkrepo.host")),
		act.req.BizId, act.req.ContentId)
	if err != nil {
		return pbcommon.ErrCode_E_TPL_SYSTEM_UNKNOWN, err.Error()
	}

	auth := &bkrepo.Auth{Token: act.viper.GetString("bkrepo.token"), UID: act.kit.User}
	if err := bkrepo.ValidateContentExistence(contentURL, auth, act.viper.GetDuration("bkrepo.timeout")); err != nil {
		return pbcommon.ErrCode_E_TPL_SYSTEM_UNKNOWN, err.Error()
	}
	return pbcommon.ErrCode_E_OK, "OK"
}

func (act *UpdateAction) updateConfigTemplateVersion() (pbcommon.ErrCode, string) {
	req := &pbdatamanager.UpdateConfigTemplateVersionReq{
		Seq:         act.kit.Rid,
		BizId:       act.req.BizId,
		VersionId:   act.req.VersionId,
		ContentId:   act.req.ContentId,
		ContentSize: act.req.ContentSize,
		Memo:        act.req.Memo,
		Operator:    act.kit.User,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("datamanager.callTimeout"))
	defer cancel()

	logger.V(4).Infof("UpdateConfigTemplateVersion[%s]| request to DataManager, %+v", req.Seq, req)

	resp, err := act.dataMgrCli.UpdateConfigTemplateVersion(ctx, req)
	if err != nil {
		return pbcommon.ErrCode_E_TPL_SYSTEM_UNKNOWN,
			fmt.Sprintf("request to DataManager UpdateConfigTemplateVersion, %+v", err)
	}
	if resp.Code != pbcommon.ErrCode_E_OK {
		return resp.Code, resp.Message
	}

	// audit here on template version updated.
	audit.Audit(int32(pbcommon.SourceType_ST_TEMPLATE_VERSION), int32(pbcommon.SourceOpType_SOT_UPDATE),
		act.req.BizId, act.req.VersionId, act.kit.User, "")

	return pbcommon.ErrCode_E_OK, "OK"
}

// Do makes the workflows of this action base on input messages.
func (act *UpdateAction) Do() error {
	// validate existence.
	if errCode, errMsg := act.validateContent(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}

	// update config template version.
	if errCode, errMsg := act.updateConfigTemplateVersion(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}
