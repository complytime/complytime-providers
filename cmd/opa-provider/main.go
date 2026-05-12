// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/server"
)

func main() {
	opaProvider := server.New(server.ServerOptions{})
	provider.Serve(opaProvider)
}
