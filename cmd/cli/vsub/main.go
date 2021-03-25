/*
Copyright 2019 The Volcano Authors.

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
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

	"pkg.yezhisheng.me/volcano/cmd/cli/util"
	"pkg.yezhisheng.me/volcano/pkg/cli/vsub"
)

var logFlushFreq = pflag.Duration("log-flush-frequency", 5*time.Second, "Maximum number of seconds between log flushes")

func main() {
	// flag.InitFlags()
	klog.InitFlags(nil)

	// The default klog flush interval is 30 seconds, which is frighteningly long.
	go wait.Until(klog.Flush, *logFlushFreq, wait.NeverStop)
	defer klog.Flush()

	rootCmd := cobra.Command{
		Use:   "vsub",
		Short: "submit a job",
		Long:  `submit a job with specified name and arguments. yaml/json file is not accepted, you may refer to kubectl.`,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckError(cmd, vsub.RunJob())
		},
	}

	jobSubCmd := &rootCmd
	vsub.InitRunFlags(jobSubCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Failed to execute vsub: %v\n", err)
		os.Exit(-2)
	}
}
