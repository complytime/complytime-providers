// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/complytime/complytime-providers/cmd/openscap-provider/server"
	"github.com/complytime/complyctl/pkg/provider"
)

func main() {
	openSCAPProvider := server.New()
	provider.Serve(openSCAPProvider)
}
