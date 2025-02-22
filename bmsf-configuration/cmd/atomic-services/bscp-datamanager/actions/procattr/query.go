/*
Tencent is pleased to support the open source community by making Blueking Container Service available.
Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
Licensed under the MIT License (the "License"); you may not use this file except
in compliance with the License. You may obtain a copy of the License at
http://opensource.org/licenses/MIT
Unless required by applicable law or agreed to in writing, software distributed under
the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
either express or implied. See the License for the specific language governing permissions and
limitations under the License.
*/

package procattr

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/spf13/viper"

	"bk-bscp/internal/database"
	"bk-bscp/internal/dbsharding"
	pbcommon "bk-bscp/internal/protocol/common"
	pb "bk-bscp/internal/protocol/datamanager"
	"bk-bscp/pkg/common"
)

// QueryAction is procattr query action object.
type QueryAction struct {
	ctx   context.Context
	viper *viper.Viper
	smgr  *dbsharding.ShardingManager

	req  *pb.QueryHostProcAttrReq
	resp *pb.QueryHostProcAttrResp

	sd *dbsharding.ShardingDB

	procAttr database.ProcAttr
}

// NewQueryAction creates new QueryAction.
func NewQueryAction(ctx context.Context, viper *viper.Viper, smgr *dbsharding.ShardingManager,
	req *pb.QueryHostProcAttrReq, resp *pb.QueryHostProcAttrResp) *QueryAction {
	action := &QueryAction{ctx: ctx, viper: viper, smgr: smgr, req: req, resp: resp}

	action.resp.Seq = req.Seq
	action.resp.Code = pbcommon.ErrCode_E_OK
	action.resp.Message = "OK"

	return action
}

// Err setup error code message in response and return the error.
func (act *QueryAction) Err(errCode pbcommon.ErrCode, errMsg string) error {
	act.resp.Code = errCode
	act.resp.Message = errMsg
	return errors.New(errMsg)
}

// Input handles the input messages.
func (act *QueryAction) Input() error {
	if err := act.verify(); err != nil {
		return act.Err(pbcommon.ErrCode_E_DM_PARAMS_INVALID, err.Error())
	}
	return nil
}

// Output handles the output messages.
func (act *QueryAction) Output() error {
	procAttr := &pbcommon.ProcAttr{
		CloudId:      act.procAttr.CloudID,
		Ip:           act.procAttr.IP,
		BizId:        act.procAttr.BizID,
		AppId:        act.procAttr.AppID,
		Path:         act.procAttr.Path,
		Labels:       act.procAttr.Labels,
		Creator:      act.procAttr.Creator,
		Memo:         act.procAttr.Memo,
		State:        act.procAttr.State,
		LastModifyBy: act.procAttr.LastModifyBy,
		CreatedAt:    act.procAttr.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    act.procAttr.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	act.resp.Data = procAttr
	return nil
}

func (act *QueryAction) verify() error {
	var err error

	if err = common.ValidateString("cloud_id", act.req.CloudId,
		database.BSCPNOTEMPTY, database.BSCPNAMELENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateString("ip", act.req.Ip,
		database.BSCPNOTEMPTY, database.BSCPNORMALSTRLENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateString("biz_id", act.req.BizId,
		database.BSCPNOTEMPTY, database.BSCPIDLENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateString("app_id", act.req.AppId,
		database.BSCPNOTEMPTY, database.BSCPIDLENLIMIT); err != nil {
		return err
	}

	act.req.Path = filepath.Clean(act.req.Path)
	if err = common.ValidateString("path", act.req.Path,
		database.BSCPNOTEMPTY, database.BSCPCFGFPATHLENLIMIT); err != nil {
		return err
	}
	return nil
}

func (act *QueryAction) queryProcAttr() (pbcommon.ErrCode, string) {
	err := act.sd.DB().
		Where(&database.ProcAttr{
			CloudID: act.req.CloudId,
			IP:      act.req.Ip,
			BizID:   act.req.BizId,
			AppID:   act.req.AppId,
			Path:    act.req.Path,
		}).
		Last(&act.procAttr).Error

	// not found.
	if err == dbsharding.RECORDNOTFOUND {
		return pbcommon.ErrCode_E_DM_NOT_FOUND, "procattr non-exist."
	}
	if err != nil {
		return pbcommon.ErrCode_E_DM_DB_EXEC_ERR, err.Error()
	}
	return pbcommon.ErrCode_E_OK, ""
}

// Do makes the workflows of this action base on input messages.
func (act *QueryAction) Do() error {
	// BSCP sharding db.
	sd, err := act.smgr.ShardingDB(dbsharding.BSCPDBKEY)
	if err != nil {
		return act.Err(pbcommon.ErrCode_E_DM_ERR_DBSHARDING, err.Error())
	}
	act.sd = sd

	// query procattr.
	if errCode, errMsg := act.queryProcAttr(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}
