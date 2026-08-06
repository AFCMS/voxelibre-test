// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package luanti

type ServerVersion struct {
	Version    string
	Entrypoint string
}

var supportedServerVersions = []ServerVersion{
	{
		Version:    "5.14.0",
		Entrypoint: "/work/dist/luanti-5.14.0-server/bin/luantiserver",
	},
	{
		Version:    "5.15.2",
		Entrypoint: "/work/dist/luanti-5.15.2-server/bin/luantiserver",
	},
	{
		Version:    "5.16.1",
		Entrypoint: "/work/dist/luanti-5.16.1-server/bin/luantiserver",
	},
}

func SupportedServerVersions() []ServerVersion {
	versions := make([]ServerVersion, len(supportedServerVersions))
	copy(versions, supportedServerVersions)
	return versions
}
