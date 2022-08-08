package main

import (
	"secrets-sync.operators.infra/controllers"
	"secrets-sync.operators.infra/helpers"
	"secrets-sync.operators.infra/types"

	"k8s.io/klog/v2"
)

var (
	builtBy   string
	commit    string
	date      string
	name      string
	target    string
	timestamp string
	version   string
)

func main() {
	op := &types.OperatorInfo{
		BuiltBy:   builtBy,
		Commit:    commit,
		Date:      date,
		Name:      name,
		Target:    target,
		Timestamp: timestamp,
		Version:   version,
	}

	// Initialisation client instance
	clientSet, configSet, err := helpers.NewConfigSet(op).GetClientSet()
	if err != nil {
		klog.Fatalln(err)
	}

	// Now let's start the controller
	stop := make(chan struct{})
	defer close(stop)
	go controllers.NewController(clientSet, types.ConfigSet(*configSet)).WatchSecrets().Run(1, stop)

	// Wait forever
	select {}
}
