package update

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	settingAuto      = "auto_update"
	settingLastCheck = "update_last_check"
	settingLastApply = "update_last_apply"
	settingLatest    = "update_latest_version"
	settingError     = "update_last_error"
)

// ErrRateLimited means a check or apply was requested again before the floor.
var ErrRateLimited = errors.New("rate limited")

// Options configures the updater. The release API origin is chosen by the
// caller. Production passes https://api.github.com and does not read it from
// the environment.
type Options struct {
	Version    string
	Owner      string
	Repo       string
	BaseURL    string
	Interval   time.Duration
	ManualMin  time.Duration
	Unit       string
	Restart    string
	BinaryPath string
	PublicKey  string
	OS         string
	Arch       string
	Now        func() time.Time
}

// Status is the admin view of update settings.
type Status struct {
	AutoUpdate        bool
	LastCheck         time.Time
	LastCheckSet      bool
	CurrentVersion    string
	LatestAvailable   string
	RollbackAvailable bool
	LastError         string
	Interval          time.Duration
	RestartMode       string
	Unit              string
	Verification      string
	Repository        string
	UpdateAvailable   bool
}

// Result is one check or apply.
type Result struct {
	Status          Status
	Applied         bool
	UpdateAvailable bool
	Latest          string
	Skipped         bool
}

// Service persists the auto-update toggle and installs verified releases.
type Service struct {
	db        *sql.DB
	Source    ReleaseSource
	Restarter Restarter
	version   string
	owner     string
	repo      string
	interval  time.Duration
	manualMin time.Duration
	unit      string
	restart   string
	binary    string
	os        string
	arch      string
	pub       []byte
	keyErr    error
	Now       func() time.Time
	mu        sync.Mutex
}

// New builds an updater. Auto-update stays off until an admin enables it.
func New(db *sql.DB, opt Options) *Service {
	owner := opt.Owner
	if owner == "" {
		owner = defaultOwner
	}
	repo := opt.Repo
	if repo == "" {
		repo = defaultRepo
	}
	goos := opt.OS
	if goos == "" {
		goos = runtime.GOOS
	}
	arch := opt.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}
	interval := opt.Interval
	if interval <= 0 {
		interval = 12 * time.Hour
	}
	if interval < 6*time.Hour {
		interval = 6 * time.Hour
	}
	if interval > 24*time.Hour {
		interval = 24 * time.Hour
	}
	manual := opt.ManualMin
	if manual <= 0 {
		manual = time.Minute
	}
	unit := opt.Unit
	if unit == "" || !ValidUnit(unit) {
		unit = "altas-panel"
	}
	restart := opt.Restart
	if restart != "none" {
		restart = "systemd"
	}
	version := opt.Version
	if version == "" {
		version = "v0.2.0"
	}
	pub, keyErr := ParsePublicKey(opt.PublicKey)
	svc := &Service{
		db:        db,
		version:   version,
		owner:     owner,
		repo:      repo,
		interval:  interval,
		manualMin: manual,
		unit:      unit,
		restart:   restart,
		binary:    opt.BinaryPath,
		os:        goos,
		arch:      arch,
		pub:       pub,
		keyErr:    keyErr,
		Now:       opt.Now,
	}
	if opt.BaseURL != "" || opt.Owner != "" || opt.Repo != "" || opt.OS != "" || opt.Arch != "" {
		svc.Source = &Client{
			BaseURL: opt.BaseURL,
			Owner:   owner,
			Repo:    repo,
			OS:      goos,
			Arch:    arch,
		}
	} else {
		svc.Source = &Client{Owner: owner, Repo: repo, OS: goos, Arch: arch}
	}
	if restart == "none" {
		svc.Restarter = noopRestart{}
	} else {
		svc.Restarter = Systemd{Unit: unit}
	}
	return svc
}

// Run checks for a release on a slow cadence. It does not call GitHub on
// every tick: Check returns immediately until the saved interval has elapsed
// and the admin has enabled auto-update.
func (s *Service) Run(ctx context.Context) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			_, _ = s.Check(ctx, false)
		case <-ticker.C:
			_, _ = s.Check(ctx, false)
		}
	}
}

// Status reads the saved toggle and the current backup.
func (s *Service) Status(ctx context.Context) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status(ctx)
}

// SetAutoUpdate persists the admin toggle. The default remains off.
func (s *Service) SetAutoUpdate(ctx context.Context, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value := "0"
	if enabled {
		value = "1"
	}
	return s.put(ctx, settingAuto, value)
}

// Check contacts the official release API.
// manual is the admin Check now action. Scheduled checks are skipped while
// auto-update is off or the interval has not elapsed. When auto-update is on
// and the verified release is newer, Check installs it.
func (s *Service) Check(ctx context.Context, manual bool) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load(ctx)
	if err != nil {
		return Result{}, err
	}
	now := s.now()
	if !manual {
		if !st.auto {
			return Result{Skipped: true}, nil
		}
		if st.checked && now.Sub(st.checkedAt) < s.interval {
			return Result{Skipped: true, Latest: st.latest}, nil
		}
	} else if st.checked && now.Sub(st.checkedAt) < s.manualMin {
		return Result{}, ErrRateLimited
	}
	if err := s.put(ctx, settingLastCheck, now.UTC().Format(time.RFC3339)); err != nil {
		return Result{}, err
	}
	return s.fetchAndMaybeInstall(ctx, st.auto)
}

// Apply downloads the latest release, verifies it, and installs it when it
// is newer. Apply does not require the auto-update toggle.
func (s *Service) Apply(ctx context.Context) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load(ctx)
	if err != nil {
		return Result{}, err
	}
	now := s.now()
	if st.applied && now.Sub(st.appliedAt) < s.manualMin {
		return Result{}, ErrRateLimited
	}
	if err := s.put(ctx, settingLastApply, now.UTC().Format(time.RFC3339)); err != nil {
		return Result{}, err
	}
	rel, err := s.fetchVerified(ctx)
	if err != nil {
		return Result{}, err
	}
	res := Result{Latest: rel.Tag, UpdateAvailable: Newer(rel.Tag, s.version)}
	if res.UpdateAvailable {
		if err := s.install(ctx, rel); err != nil {
			_ = s.put(ctx, settingError, clientUpdateError(err))
			return Result{}, err
		}
		res.Applied = true
	}
	if err := s.put(ctx, settingLatest, rel.Tag); err != nil {
		return Result{}, err
	}
	_ = s.put(ctx, settingError, "")
	res.Status, err = s.status(ctx)
	if err != nil {
		return Result{}, err
	}
	res.Status.UpdateAvailable = res.UpdateAvailable
	return res, nil
}

// Rollback restores the previous binary and restarts the panel.
func (s *Service) Rollback(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.binaryPath()
	if err != nil {
		return err
	}
	if err := RestoreBackup(path); err != nil {
		return err
	}
	if s.Restarter == nil {
		return ErrRestart
	}
	if err := s.Restarter.Restart(ctx); err != nil {
		return err
	}
	_ = s.put(ctx, settingError, "")
	return nil
}

func (s *Service) fetchAndMaybeInstall(ctx context.Context, auto bool) (Result, error) {
	rel, err := s.fetchVerified(ctx)
	if err != nil {
		return Result{}, err
	}
	res := Result{Latest: rel.Tag, UpdateAvailable: Newer(rel.Tag, s.version)}
	if auto && res.UpdateAvailable {
		if err := s.install(ctx, rel); err != nil {
			_ = s.put(ctx, settingError, clientUpdateError(err))
			return Result{}, err
		}
		res.Applied = true
	}
	if err := s.put(ctx, settingLatest, rel.Tag); err != nil {
		return Result{}, err
	}
	_ = s.put(ctx, settingError, "")
	res.Status, err = s.status(ctx)
	if err != nil {
		return Result{}, err
	}
	res.Status.UpdateAvailable = res.UpdateAvailable
	return res, nil
}

func (s *Service) fetchVerified(ctx context.Context) (Release, error) {
	if s.keyErr != nil {
		_ = s.put(ctx, settingError, clientUpdateError(s.keyErr))
		return Release{}, s.keyErr
	}
	if s.Source == nil {
		return Release{}, ErrSource
	}
	rel, err := s.Source.FetchLatest(ctx)
	if err != nil {
		_ = s.put(ctx, settingError, clientUpdateError(err))
		return Release{}, err
	}
	if err := s.verify(rel); err != nil {
		_ = s.put(ctx, settingError, clientUpdateError(err))
		return Release{}, err
	}
	return rel, nil
}

func (s *Service) verify(rel Release) error {
	want, err := BinaryAssetName(s.os, s.arch)
	if err != nil {
		return err
	}
	if rel.Name != want || !IsInstallableBinary(rel.Binary) {
		return ErrNotBinary
	}
	sums, err := ParseChecksums(string(rel.Checksums))
	if err != nil {
		return err
	}
	digest, err := DigestFor(sums, rel.Name)
	if err != nil {
		return err
	}
	if err := VerifySHA256(rel.Binary, digest); err != nil {
		return err
	}
	if s.pub != nil {
		if len(rel.Signature) == 0 {
			return ErrSignature
		}
		if err := VerifyEd25519(s.pub, rel.Binary, rel.Signature); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) install(ctx context.Context, rel Release) error {
	path, err := s.binaryPath()
	if err != nil {
		return err
	}
	if err := Install(path, rel.Binary); err != nil {
		return err
	}
	if s.Restarter == nil {
		_ = RestoreBackup(path)
		return ErrRestart
	}
	if err := s.Restarter.Restart(ctx); err != nil {
		_ = RestoreBackup(path)
		_ = s.Restarter.Restart(ctx)
		return err
	}
	return nil
}

func (s *Service) binaryPath() (string, error) {
	path := s.binary
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		path = exe
	}
	return cleanBinaryPath(path)
}

type saved struct {
	auto      bool
	checked   bool
	checkedAt time.Time
	applied   bool
	appliedAt time.Time
	latest    string
	lastError string
}

func (s *Service) load(ctx context.Context) (saved, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings WHERE key IN (?, ?, ?, ?, ?)`,
		settingAuto, settingLastCheck, settingLastApply, settingLatest, settingError)
	if err != nil {
		return saved{}, err
	}
	defer rows.Close()
	var st saved
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return saved{}, err
		}
		switch key {
		case settingAuto:
			st.auto = value == "1"
		case settingLastCheck:
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				st.checked = true
				st.checkedAt = t
			}
		case settingLastApply:
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				st.applied = true
				st.appliedAt = t
			}
		case settingLatest:
			st.latest = value
		case settingError:
			st.lastError = value
		}
	}
	return st, rows.Err()
}

func (s *Service) status(ctx context.Context) (Status, error) {
	st, err := s.load(ctx)
	if err != nil {
		return Status{}, err
	}
	rollback := false
	if path, err := s.binaryPath(); err == nil {
		rollback = HasBackup(path)
	}
	verification := "sha256"
	if s.pub != nil {
		verification = "sha256+ed25519"
	}
	available := st.latest != "" && Newer(st.latest, s.version)
	return Status{
		AutoUpdate:        st.auto,
		LastCheck:         st.checkedAt,
		LastCheckSet:      st.checked,
		CurrentVersion:    s.version,
		LatestAvailable:   st.latest,
		RollbackAvailable: rollback,
		LastError:         st.lastError,
		Interval:          s.interval,
		RestartMode:       s.restart,
		Unit:              s.unit,
		Verification:      verification,
		Repository:        s.owner + "/" + s.repo,
		UpdateAvailable:   available,
	}, nil
}

func (s *Service) put(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, s.now().UTC().Format(time.RFC3339))
	return err
}

func clientUpdateError(err error) string {
	switch {
	case errors.Is(err, ErrChecksumMismatch), errors.Is(err, ErrChecksumFormat):
		return "Release checksum did not match. The binary was not replaced."
	case errors.Is(err, ErrSignature):
		return "Release signature verification failed. The binary was not replaced."
	case errors.Is(err, ErrNotBinary):
		return "Release asset is not a binary. The file was not replaced."
	case errors.Is(err, ErrNoBackup):
		return "No previous binary is available to restore."
	case errors.Is(err, ErrRestart):
		return "The new binary was not kept because the service did not restart."
	case errors.Is(err, ErrNoRelease):
		return "No official release has been published yet."
	default:
		return "Could not read the official release."
	}
}

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}
