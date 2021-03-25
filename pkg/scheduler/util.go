/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scheduler

import (
	"fmt"
	"io/ioutil"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"pkg.yezhisheng.me/volcano/pkg/scheduler/conf"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/framework"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins"
)

var defaultSchedulerConf = `
actions: "enqueue, allocate, backfill"
tiers:
- plugins:
  - name: priority
  - name: gang
- plugins:
  - name: drf
  - name: predicates
  - name: proportion
  - name: nodeorder
`

func unmarshalSchedulerConf(confStr string) ([]framework.Action, []conf.Tier, []conf.Configuration, error) {
	var actions []framework.Action

	schedulerConf := &conf.SchedulerConfiguration{}

	buf := make([]byte, len(confStr))
	copy(buf, confStr)

	if err := yaml.Unmarshal(buf, schedulerConf); err != nil {
		return nil, nil, nil, err
	}
	// Set default settings for each plugin if not set
	for i, tier := range schedulerConf.Tiers {
		// drf with hierarchy enabled
		hdrf := false
		// proportion enabled
		proprotion := false
		for j := range tier.Plugins {
			if tier.Plugins[j].Name == "drf" &&
				tier.Plugins[j].EnabledHierarchy != nil &&
				*tier.Plugins[j].EnabledHierarchy == true {
				hdrf = true
			}
			if tier.Plugins[j].Name == "proportion" {
				proprotion = true
			}
			plugins.ApplyPluginConfDefaults(&schedulerConf.Tiers[i].Plugins[j])
		}
		if hdrf && proprotion {
			return nil, nil, nil, fmt.Errorf("proportion and drf with hierarchy enabled conflicts")
		}
	}

	actionNames := strings.Split(schedulerConf.Actions, ",")
	for _, actionName := range actionNames {
		if action, found := framework.GetAction(strings.TrimSpace(actionName)); found {
			actions = append(actions, action)
		} else {
			return nil, nil, nil, fmt.Errorf("failed to found Action %s, ignore it", actionName)
		}
	}

	return actions, schedulerConf.Tiers, schedulerConf.Configurations, nil
}

func readSchedulerConf(confPath string) (string, error) {
	dat, err := ioutil.ReadFile(confPath)
	if err != nil {
		return "", err
	}
	return string(dat), nil
}
