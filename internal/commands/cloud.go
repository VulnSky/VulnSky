package commands

import (
	"vulnsky/internal/aliyun"
	"vulnsky/internal/config"
)

func loadCommandConfig(state *rootState) (config.Config, error) {
	return config.Load(state.rootDir, state.profile)
}

func loadOSSClient(state *rootState) (config.Config, aliyun.OSSClient, error) {
	cfg, err := loadCommandConfig(state)
	if err != nil {
		return cfg, nil, err
	}
	if err := cfg.ValidateOSS(); err != nil {
		return cfg, nil, err
	}
	client, err := state.factories.NewOSS(cfg)
	return cfg, client, err
}

func loadSTSClient(state *rootState) (config.Config, aliyun.STSClient, error) {
	cfg, err := loadCommandConfig(state)
	if err != nil {
		return cfg, nil, err
	}
	if err := cfg.ValidateECS(); err != nil {
		return cfg, nil, err
	}
	client, err := state.factories.NewSTS(cfg)
	return cfg, client, err
}

func newOSSSTSClient(state *rootState, cfg config.Config) (aliyun.STSClient, error) {
	stsCfg := cfg
	stsCfg.CloudAccessKeyID = cfg.OSSAccessKeyID
	stsCfg.CloudAccessKeySecret = cfg.OSSAccessKeySecret
	stsCfg.CloudRegionID = cfg.OSSRegionID
	return state.factories.NewSTS(stsCfg)
}

func loadECSClient(state *rootState) (config.Config, aliyun.ECSClient, error) {
	cfg, err := loadCommandConfig(state)
	if err != nil {
		return cfg, nil, err
	}
	if err := cfg.ValidateECS(); err != nil {
		return cfg, nil, err
	}
	client, err := state.factories.NewECS(cfg)
	return cfg, client, err
}
