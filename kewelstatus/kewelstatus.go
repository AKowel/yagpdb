package kewelstatus

import (
	"github.com/botlabs-gg/yagpdb/v2/common"
)

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Kewel Status",
		SysName:  "kewelstatus",
		Category: common.PluginCategoryFeeds,
	}
}

func RegisterPlugin() {
	common.InitSchemas("kewelstatus", DBSchemas...)
	common.RegisterPlugin(&Plugin{})
}
