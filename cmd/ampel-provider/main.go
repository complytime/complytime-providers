// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/complytime/complytime-providers/cmd/ampel-provider/server"
	"github.com/complytime/complyctl/pkg/provider"
)

func main() {
	ampelProvider := server.New()
	provider.Serve(ampelProvider)
}
