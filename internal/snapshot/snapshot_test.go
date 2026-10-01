/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0

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
package snapshot

import (
	"testing"

	chainsv1alpha2 "github.com/tazhate/chainplane/api/v1alpha2"
)

func TestBuildInitContainerImage(t *testing.T) {
	origTag := RestoreImageTag
	t.Cleanup(func() { RestoreImageTag = origTag })

	tests := []struct {
		name string
		env  string
		tag  string
		want string
	}{
		{name: "dev build default", tag: "latest", want: "ghcr.io/tazhate/chainplane/snapshot-restore:latest"},
		{name: "release build default", tag: "v0.5.0", want: "ghcr.io/tazhate/chainplane/snapshot-restore:v0.5.0"},
		{name: "env override wins", env: "registry.local/restore:pinned", tag: "v0.5.0", want: "registry.local/restore:pinned"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SNAPSHOT_RESTORE_IMAGE", tt.env)
			RestoreImageTag = tt.tag
			c := BuildInitContainer(chainsv1alpha2.ChainBitcoin, Config{})
			if c.Image != tt.want {
				t.Errorf("Image = %q, want %q", c.Image, tt.want)
			}
		})
	}
}
