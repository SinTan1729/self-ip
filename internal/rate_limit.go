// SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package internal

import (
	"net/netip"
	"time"
)

func InitBanManager() *banManager {
	b := banManager{
		records:    make(map[netip.Addr]*banRecord),
		path:       "/var/lib/self-ip/banManager.gob",
		Threshold:  5,
		BaseDelay:  5 * time.Second,
		Multiplier: 2,
		MaxDelay:   6 * time.Hour,
		ResetAfter: 30 * time.Minute,
	}
	return &b
}
