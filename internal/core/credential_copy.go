package core

// PrepareCredentialCopy returns an identity-rebound update without writing the
// vault, so callers can commit duplicated configuration and credentials together.
func (v *Vault) PrepareCredentialCopy(source, destination Profile) (CredentialUpdate, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.data.Entries[source.ID]
	if !ok || entry.Identity != credentialIdentity(source) {
		return CredentialUpdate{}, false, nil
	}
	plain, err := v.decryptEntryLocked(source.ID, entry)
	if err != nil {
		return CredentialUpdate{}, false, err
	}
	defer clear(plain)
	identity := source.Identity()
	if entry.Resolved != nil {
		identity = *entry.Resolved
	}
	protection := "master"
	if entry.Local || !v.data.Master {
		protection = "local"
	}
	return CredentialUpdate{Profile: destination, Password: string(plain), Identity: identity, Protection: protection}, true, nil
}
