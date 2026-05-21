package connector

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

const installationTableSQL = `
CREATE TABLE IF NOT EXISTS github_installation (
	installation_id BIGINT PRIMARY KEY,
	user_login_id   TEXT NOT NULL,
	account_login   TEXT NOT NULL,
	repos           JSONB NOT NULL DEFAULT '[]',
	updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS github_installation_user_login_id_idx ON github_installation (user_login_id);
`

type installationRecord struct {
	InstallationID int64
	UserLoginID    string
	AccountLogin   string
	Repos          []int64
	UpdatedAt      time.Time
}

func (gc *GHDConnector) migrateInstallationTable(ctx context.Context) error {
	_, err := gc.br.DB.Exec(ctx, installationTableSQL)
	return err
}

func (gc *GHDConnector) upsertInstallation(ctx context.Context, rec installationRecord) error {
	reposJSON, err := json.Marshal(rec.Repos)
	if err != nil {
		return err
	}
	_, err = gc.br.DB.Exec(ctx, `
		INSERT INTO github_installation (installation_id, user_login_id, account_login, repos, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (installation_id) DO UPDATE SET
			user_login_id = EXCLUDED.user_login_id,
			account_login = EXCLUDED.account_login,
			repos = EXCLUDED.repos,
			updated_at = EXCLUDED.updated_at
	`, rec.InstallationID, rec.UserLoginID, rec.AccountLogin, reposJSON, rec.UpdatedAt)
	return err
}

func (gc *GHDConnector) deleteInstallation(ctx context.Context, installationID int64) error {
	_, err := gc.br.DB.Exec(ctx, `DELETE FROM github_installation WHERE installation_id = $1`, installationID)
	return err
}

func (gc *GHDConnector) getUserLoginsForRepo(ctx context.Context, repoID int64) ([]string, error) {
	rows, err := gc.br.DB.Query(ctx, `SELECT user_login_id, repos FROM github_installation`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logins []string
	seen := make(map[string]bool)
	for rows.Next() {
		var loginID string
		var reposJSON []byte
		if err := rows.Scan(&loginID, &reposJSON); err != nil {
			return nil, err
		}
		var repos []int64
		if err := json.Unmarshal(reposJSON, &repos); err != nil {
			return nil, err
		}
		for _, id := range repos {
			if id == repoID && !seen[loginID] {
				seen[loginID] = true
				logins = append(logins, loginID)
				break
			}
		}
	}
	return logins, rows.Err()
}

func (gc *GHDConnector) getUserLoginIDByGitHubUserID(ctx context.Context, githubUserID int64) (string, error) {
	rows, err := gc.br.DB.Query(ctx, `SELECT id, metadata FROM user_login WHERE bridge_id = $1`, gc.br.ID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var metadataJSON []byte
		if err := rows.Scan(&id, &metadataJSON); err != nil {
			return "", err
		}
		var meta UserLoginMetadata
		if err := json.Unmarshal(metadataJSON, &meta); err != nil {
			continue
		}
		if meta.DatabaseID == githubUserID {
			return id, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return "", sql.ErrNoRows
}
