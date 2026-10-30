package trackerstore

// ForgetMixedProfiles forgets the profiles remembered with a mode that is
// not theirs. Before 2026-10-30 a mode switch in one EFT session reported
// the profile read before with the new mode for a moment (a Seasonal
// profile with the PvE profile's ID), and it was remembered. Such a profile
// is not in the logs (observed), while its ID is, with another mode. One a
// key is assigned to is kept.
func (d *Document) ForgetMixedProfiles(observed []Profile) bool {
	seen := make(map[string]bool, len(observed))
	ids := make(map[string]bool, len(observed))
	for _, profile := range observed {
		seen[profile.AccountID+"\x00"+profile.ProfileID+"\x00"+profile.Mode] = true
		ids[profile.AccountID+"\x00"+profile.ProfileID] = true
	}
	kept := d.Profiles[:0:0]
	for _, profile := range d.Profiles {
		mixed := !seen[profile.AccountID+"\x00"+profile.ProfileID+"\x00"+profile.Mode] && ids[profile.AccountID+"\x00"+profile.ProfileID]
		if mixed && d.TokenFor(profile.AccountID, profile.ProfileID, profile.Mode) == "" {
			continue
		}
		kept = append(kept, profile)
	}
	if len(kept) == len(d.Profiles) {
		return false
	}
	d.Profiles = kept
	return true
}
