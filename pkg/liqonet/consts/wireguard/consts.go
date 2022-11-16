// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wireguardconsts

const (
	// EndpointIP is the key of the endpointIP entry in back-end map.
	EndpointIP = "endpointIP"
	// DriverName is the name of the driver.
	DriverName = "wireguard"
	// PrivateKey is the key of private for the secret containing the wireguard keys.
	PrivateKey = "privateKey"
	// AllowedIPs is the key of the allowedIPs entry in the back-end map.
	AllowedIPs = "allowedIPs"
	// KeysName is the name of the secret that contains the public key used by wireguard.
	KeysName = "wireguard-pubkey"
)
