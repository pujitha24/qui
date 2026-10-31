// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/models"
)

func TestCrossSeedBlocklistStore_UpsertListDelete(t *testing.T) {
	db := setupCrossSeedTestDB(t)
	ctx := context.Background()

	instanceStore, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	instance, err := instanceStore.Create(ctx, "Test Instance", "http://localhost:8080", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)

	store := models.NewCrossSeedBlocklistStore(db)
	infohash := "63E07FF523710CA268567DAD344CE1E0E6B7E8A3"

	entry, err := store.Upsert(ctx, &models.CrossSeedBlocklistEntry{
		InstanceID: instance.ID,
		InfoHash:   infohash,
		Note:       "  bad files ",
	})
	require.NoError(t, err)
	assert.Equal(t, strings.ToLower(infohash), entry.InfoHash)
	assert.Equal(t, "bad files", entry.Note)

	list, err := store.List(ctx, instance.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, entry.InfoHash, list[0].InfoHash)

	blockedHash, blocked, err := store.FindBlocked(ctx, instance.ID, []string{"deadbeef", strings.ToUpper(entry.InfoHash)})
	require.NoError(t, err)
	assert.True(t, blocked)
	assert.Equal(t, entry.InfoHash, blockedHash)

	require.NoError(t, store.Delete(ctx, instance.ID, entry.InfoHash))

	list, err = store.List(ctx, instance.ID)
	require.NoError(t, err)
	assert.Empty(t, list)

	err = store.Delete(ctx, instance.ID, entry.InfoHash)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestCrossSeedBlocklistStore_FindBlockedHashForms(t *testing.T) {
	db := setupCrossSeedTestDB(t)
	ctx := context.Background()

	instanceStore, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	instance, err := instanceStore.Create(ctx, "Test Instance", "http://localhost:8080", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)

	v1 := "63e07ff523710ca268567dad344ce1e0e6b7e8a3"
	v2 := "3cee10bbd1f3a5e9c7b0a4d2f6e8914b5c7d0a2e1f3b5d7c9e0a2b4d6f8c1e3a"
	require.Len(t, v2, 64)

	tests := []struct {
		name    string
		stored  string
		lookup  []string
		blocked bool
	}{
		{"v1 stored, v1 lookup", v1, []string{v1}, true},
		{"hybrid stored as v1", v1, []string{v1, v2}, true},
		{"hybrid stored as truncated v2", v2[:40], []string{v1, v2}, true},
		{"v2-only stored as truncated v2", v2[:40], []string{v2}, true},
		{"v2-only stored as full v2", v2, []string{v2}, true},
		{"unrelated hash", v1, []string{v2}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := models.NewCrossSeedBlocklistStore(db)
			_, err := store.Upsert(ctx, &models.CrossSeedBlocklistEntry{InstanceID: instance.ID, InfoHash: tt.stored})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Delete(ctx, instance.ID, tt.stored)) })

			_, blocked, err := store.FindBlocked(ctx, instance.ID, tt.lookup)
			require.NoError(t, err)
			assert.Equal(t, tt.blocked, blocked)
		})
	}
}
