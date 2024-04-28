/*
 * Copyright (c) 2024 Yunshan Networks
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package huawei

import (
	"fmt"

	cloudcommon "github.com/deepflowio/deepflow/server/controller/cloud/common"
	"github.com/deepflowio/deepflow/server/controller/cloud/model"
	"github.com/deepflowio/deepflow/server/controller/common"
)

func (h *HuaWei) getNetworks() ([]model.Network, []model.Subnet, []model.VInterface, error) {
	var networks []model.Network
	var subnets []model.Subnet
	var vifs []model.VInterface

	requiredAttrs := []string{"id", "name", "cidr", "vpc_id", "availability_zone"}
	for project, token := range h.projectTokenMap {
		jNetworks, err := h.getRawData(newRawDataGetContext(
			fmt.Sprintf("https://vpc.%s.%s/v1/%s/subnets", project.name, h.config.Domain, project.id), token.token, "subnets", pageQueryMethodMarker,
		))
		if err != nil {
			return nil, nil, nil, err
		}

		regionLcuuid := h.projectNameToRegionLcuuid(project.name)
		for i := range jNetworks {
			jn := jNetworks[i]
			id := jn.Get("id").MustString()
			name := jn.Get("name").MustString()
			if !cloudcommon.CheckJsonAttributes(jn, requiredAttrs) {
				log.Infof("exclude network: %s, missing attr", name)
				continue
			}
			vpcID := jn.Get("vpc_id").MustString()
			azLcuuid := h.toolDataSet.azNameToAZLcuuid[jn.Get("availability_zone").MustString()]
			network := model.Network{
				Lcuuid:         id,
				Name:           name,
				SegmentationID: 1,
				Shared:         false,
				External:       false,
				NetType:        common.NETWORK_TYPE_LAN,
				VPCLcuuid:      vpcID,
				AZLcuuid:       azLcuuid,
				RegionLcuuid:   regionLcuuid,
			}
			networks = append(networks, network)
			h.toolDataSet.azLcuuidToResourceNum[azLcuuid]++
			h.toolDataSet.regionLcuuidToResourceNum[regionLcuuid]++
			h.toolDataSet.lcuuidToNetwork[id] = network

			cidr := jn.Get("cidr").MustString()
			if cidr != "" {
				subnetLcuuid := common.GenerateUUIDByOrgID(h.orgID, id+cidr)
				subnet := model.Subnet{
					Lcuuid:        subnetLcuuid,
					Name:          name,
					CIDR:          cidr,
					NetworkLcuuid: id,
					VPCLcuuid:     vpcID,
				}
				subnets = append(subnets, subnet)
				h.toolDataSet.networkLcuuidToSubnets[id] = append(h.toolDataSet.networkLcuuidToSubnets[id], subnet)
			}
			cidrV6 := jn.Get("cidr_v6").MustString()
			if cidrV6 != "" {
				subnetLcuuid := common.GenerateUUIDByOrgID(h.orgID, id+cidrV6)
				subnet := model.Subnet{
					Lcuuid:        subnetLcuuid,
					Name:          name + "_v6",
					CIDR:          cidrV6,
					NetworkLcuuid: id,
					VPCLcuuid:     vpcID,
				}
				subnets = append(subnets, subnet)
				h.toolDataSet.networkLcuuidToSubnets[id] = append(h.toolDataSet.networkLcuuidToSubnets[id], subnet)
			}

			vifs = append(
				vifs,
				model.VInterface{
					Lcuuid:        common.GenerateUUIDByOrgID(h.orgID, id+vpcID),
					Type:          common.VIF_TYPE_LAN,
					Mac:           common.VIF_DEFAULT_MAC,
					DeviceType:    common.VIF_DEVICE_TYPE_VROUTER,
					DeviceLcuuid:  common.GenerateUUIDByOrgID(h.orgID, vpcID),
					NetworkLcuuid: id,
					VPCLcuuid:     vpcID,
					RegionLcuuid:  regionLcuuid,
				},
			)
			h.toolDataSet.networkLcuuidToCIDR[id] = cidr
			h.toolDataSet.networkVPCLcuuidToAZLcuuids[vpcID] = append(h.toolDataSet.networkVPCLcuuidToAZLcuuids[vpcID], azLcuuid)
			neutronSubnetID, ok := jn.CheckGet("neutron_subnet_id")
			if ok {
				h.toolDataSet.neutronSubnetIDToNetwork[neutronSubnetID.MustString()] = network
			}
		}
	}
	return networks, subnets, vifs, nil
}
