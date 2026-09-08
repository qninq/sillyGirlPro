package core

import (
	"fmt"

	"github.com/qninq/sillyGirlPro/core/common"
	"github.com/qninq/sillyGirlPro/utils"
)

var IsCdle = false

func init() {
	if sillyGirl.GetString("is_cdle") == "silly8023" {
		IsCdle = true
	}
	if !IsCdle {
		return
	}
	AddCommand([]*common.Function{ //认证订阅
		{
			Admin: true,
			Rules: []string{"identify sublink [地址] [组织]"},
			Handle: func(s common.Sender) interface{} {
				address := s.Get(0)
				organization := s.Get(1)
				// machine_id := s.Get(1)
				if err := CheckPluginAddress(address); err != nil {
					return err
				}
				str, err := EncryptByAes(utils.JsonMarshal(common.PluginPublisher{
					Address:      address,
					Organization: organization,
					Identified:   true,
					// MachineID:    machine_id,
				}))
				if err != nil {
					return err
				}
				sublink := fmt.Sprintf("link://%s", str)
				return sublink
			},
		},
	})
}
