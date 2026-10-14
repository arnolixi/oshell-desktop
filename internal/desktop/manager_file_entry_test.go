package desktop

import "testing"

func TestManagerSSHFileEntryUsesSavedCredentialAndKeepsProfile(t *testing.T) {
	fileApp, profile, accepted := savedConnectionFixture(t)
	hub := &applicationHub{fileWindow: fileApp, store: fileApp.store, config: fileApp.config, vault: fileApp.vault, trust: fileApp.trust}
	fileApp.hub = hub
	source := newApp(fileApp.store, fileApp.config)
	source.hub = hub
	source.vault = fileApp.vault
	if err := fileApp.vault.PutFor(profile, "saved-fixture-password", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	source.openSavedFileSession(profile)
	waitOperator(t, fileApp, func() bool {
		return len(fileApp.tabs) == 1 && fileApp.tabs[0].active.files != nil && !fileApp.tabs[0].active.files.busy && fileApp.tabs[0].active.files.client != nil
	})
	if len(source.tabs) != 0 || accepted.Load() != 1 || fileApp.tabs[0].active.term != nil || fileApp.config.Profiles[0].Kind != "ssh" {
		t.Fatal("file entry changed session type or terminal workspace")
	}
}
