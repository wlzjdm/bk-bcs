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

package appinstance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"

	"bk-bscp/internal/database"
	"bk-bscp/internal/dbsharding"
	pbcommon "bk-bscp/internal/protocol/common"
	pb "bk-bscp/internal/protocol/datamanager"
	"bk-bscp/internal/strategy"
	"bk-bscp/pkg/common"
)

// UpdateAction is appinstance update action object.
type UpdateAction struct {
	ctx   context.Context
	viper *viper.Viper
	smgr  *dbsharding.ShardingManager

	req  *pb.UpdateAppInstanceReq
	resp *pb.UpdateAppInstanceResp

	sd *dbsharding.ShardingDB
}

// NewUpdateAction creates new UpdateAction.
func NewUpdateAction(ctx context.Context, viper *viper.Viper, smgr *dbsharding.ShardingManager,
	req *pb.UpdateAppInstanceReq, resp *pb.UpdateAppInstanceResp) *UpdateAction {
	action := &UpdateAction{ctx: ctx, viper: viper, smgr: smgr, req: req, resp: resp}

	action.resp.Seq = req.Seq
	action.resp.Code = pbcommon.ErrCode_E_OK
	action.resp.Message = "OK"

	return action
}

// Err setup error code message in response and return the error.
func (act *UpdateAction) Err(errCode pbcommon.ErrCode, errMsg string) error {
	act.resp.Code = errCode
	act.resp.Message = errMsg
	return errors.New(errMsg)
}

// Input handles the input messages.
func (act *UpdateAction) Input() error {
	if err := act.verify(); err != nil {
		return act.Err(pbcommon.ErrCode_E_DM_PARAMS_INVALID, err.Error())
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
	if err = common.ValidateString("app_id", act.req.AppId,
		database.BSCPNOTEMPTY, database.BSCPIDLENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateString("cloud_id", act.req.CloudId,
		database.BSCPNOTEMPTY, database.BSCPIDLENLIMIT); err != nil {
		return err
	}
	if err = common.ValidateString("ip", act.req.Ip,
		database.BSCPNOTEMPTY, database.BSCPNORMALSTRLENLIMIT); err != nil {
		return err
	}

	if err = common.ValidateString("labels", act.req.Labels,
		database.BSCPEMPTY, database.BSCPLABELSSIZELIMIT); err != nil {
		return err
	}
	if len(act.req.Labels) == 0 {
		act.req.Labels = strategy.EmptySidecarLabels
	}
	if act.req.Labels != strategy.EmptySidecarLabels {
		labels := strategy.SidecarLabels{}
		if err := json.Unmarshal([]byte(act.req.Labels), &labels); err != nil {
			return fmt.Errorf("invalid input data, labels[%+v], %+v", act.req.Labels, err)
		}
	}

	act.req.Path = filepath.Clean(act.req.Path)
	if err = common.ValidateString("path", act.req.Path,
		database.BSCPNOTEMPTY, database.BSCPCFGFPATHLENLIMIT); err != nil {
		return err
	}
	return nil
}

func (act *UpdateAction) updateAppInstance() (pbcommon.ErrCode, string) {
	ups := map[string]interface{}{
		"Labels": act.req.Labels,
		"State":  act.req.State,
	}

	err := act.sd.DB().
		Model(&database.AppInstance{}).
		Where(&database.AppInstance{
			BizID:   act.req.BizId,
			AppID:   act.req.AppId,
			CloudID: act.req.CloudId,
			IP:      act.req.Ip,
			Path:    act.req.Path,
		}).
		Updates(ups).Error

	if err != nil {
		return pbcommon.ErrCode_E_DM_DB_EXEC_ERR, err.Error()
	}
	return pbcommon.ErrCode_E_OK, ""
}

// Do makes the workflows of this action base on input messages.
func (act *UpdateAction) Do() error {
	// business sharding db.
	sd, err := act.smgr.ShardingDB(act.req.BizId)
	if err != nil {
		return act.Err(pbcommon.ErrCode_E_DM_ERR_DBSHARDING, err.Error())
	}
	act.sd = sd

	// update appinstance.
	if errCode, errMsg := act.updateAppInstance(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}
