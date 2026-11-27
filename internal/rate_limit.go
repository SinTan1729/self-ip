// SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package internal

import (
	"encoding/gob"
	"errors"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"time"
)

func InitBanManager() (*BanManager, error) {
	b := BanManager{
		records:    make(map[netip.Addr]*banRecord),
		path:       "./state/banManager.gob",
		Threshold:  5,
		BaseDelay:  30 * time.Second,
		Multiplier: 2,
		MaxDelay:   7 * 24 * time.Hour,
		ResetAfter: 24 * time.Hour,
	}

	if err := b.Sync(false); err != nil {
		return nil, err
	}

	return &b, nil
}

func (m *BanManager) Sync(push bool) error {
	m.diskMu.Lock()
	defer m.diskMu.Unlock()

	if push {
		m.mu.RLock()
		records := make(map[string]*banRecord, len(m.records))

		for ip, record := range m.records {
			copy := *record
			records[ip.String()] = &copy
		}
		m.mu.RUnlock()

		return m.writeRecords(records)
	}

	f, err := os.Open(m.path)
	if errors.Is(err, os.ErrNotExist) {
		m.mu.Lock()
		m.records = make(map[netip.Addr]*banRecord)
		m.mu.Unlock()
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	var records map[string]*banRecord
	if err := gob.NewDecoder(f).Decode(&records); err != nil {
		return err
	}

	loaded := make(map[netip.Addr]*banRecord, len(records))

	for ip, record := range records {
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			continue
		}
		loaded[addr.Unmap()] = record
	}

	m.mu.Lock()
	m.records = loaded
	m.mu.Unlock()

	return nil
}

func (m *BanManager) writeRecords(records map[string]*banRecord) error {
	dir := filepath.Dir(m.path)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	tmp := m.path + ".tmp"

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	if err := gob.NewEncoder(f).Encode(records); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, m.path); err != nil {
		os.Remove(tmp)
		return err
	}

	// Sync the directory to make the rename durable.
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()

	return d.Sync()
}

func (m *BanManager) IsBanned(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	addr = addr.Unmap()

	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.records[addr]
	return ok && time.Now().Before(record.BannedUntil)
}

func (m *BanManager) ProcessBan(ip string) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		log.Printf(Red+"Invalid IP address: %v"+Reset, err)
		return
	}
	ipAddr := addr.Unmap()

	if m.Threshold == 0 || m.Multiplier == 0 ||
		m.BaseDelay <= 0 || m.MaxDelay <= 0 {
		log.Println(Red + "Invalid ban manager configuration" + Reset)
		return
	}

	m.diskMu.Lock()
	defer m.diskMu.Unlock()

	m.mu.Lock()

	now := time.Now()

	record, ok := m.records[ipAddr]
	if !ok {
		record = &banRecord{}
		m.records[ipAddr] = record
	}

	wasBanned := now.Before(record.BannedUntil)
	previousBanUntil := record.BannedUntil

	// Reset escalation after inactivity.
	if !record.LastFailed.IsZero() &&
		m.ResetAfter > 0 &&
		now.Sub(record.LastFailed) > m.ResetAfter {
		record.Failures = 0
		record.BanLevel = 0
		record.BannedUntil = time.Time{}
		wasBanned = false
	}

	record.LastFailed = now
	record.Failures++

	if record.Failures < m.Threshold {
		m.mu.Unlock()
		return
	}

	record.Failures -= m.Threshold

	if record.BanLevel < ^uint32(0) {
		record.BanLevel++
	}

	duration := m.BaseDelay

	for i := uint32(1); i < record.BanLevel; i++ {
		if duration >= m.MaxDelay/time.Duration(m.Multiplier) {
			duration = m.MaxDelay
			break
		}
		duration *= time.Duration(m.Multiplier)
	}

	if duration > m.MaxDelay {
		duration = m.MaxDelay
	}

	record.BannedUntil = now.Add(duration)

	if wasBanned {
		log.Printf(
			"IP ban extended: ip=%s level=%d duration=%s previous_expiry=%s new_expiry=%s",
			ip,
			record.BanLevel,
			duration,
			previousBanUntil.Format(time.RFC3339),
			record.BannedUntil.Format(time.RFC3339),
		)
	} else {
		log.Printf(
			"IP banned: ip=%s level=%d duration=%s expires=%s",
			ip,
			record.BanLevel,
			duration,
			record.BannedUntil.Format(time.RFC3339),
		)
	}

	// Snapshot while holding the lock.
	records := make(map[string]*banRecord, len(m.records))

	for addr, r := range m.records {
		copy := *r
		records[addr.String()] = &copy
	}

	m.mu.Unlock()

	if err := m.writeRecords(records); err != nil {
		log.Printf(Red+"Failed to persist ban records: %v"+Reset, err)
	}
}

func (m *BanManager) Reset(ip string) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		log.Printf(Red+"Invalid IP address: %v"+Reset, err)
		return
	}

	addr = addr.Unmap()

	m.diskMu.Lock()
	defer m.diskMu.Unlock()

	m.mu.Lock()

	record, ok := m.records[addr]
	if !ok {
		m.mu.Unlock()
		return
	}

	delete(m.records, addr)
	log.Printf(
		"IP ban record reset after successful authentication: ip=%s previous_level=%d previous_failures=%d",
		addr,
		record.BanLevel,
		record.Failures,
	)

	records := make(map[string]*banRecord, len(m.records))
	for ip, record := range m.records {
		copy := *record
		records[ip.String()] = &copy
	}

	m.mu.Unlock()

	if err := m.writeRecords(records); err != nil {
		log.Printf(Red+"Failed to persist ban reset: %v"+Reset, err)
	}
}
