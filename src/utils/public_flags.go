package utils

// PublicFlagsToNames converts the Discord public_flags bitset into semantic labels.
func PublicFlagsToNames(flag int) []string {
	flags := []string{}

	if flag&1 != 0 {
		flags = append(flags, "Discord_Employee")
	}
	if flag&262144 != 0 {
		flags = append(flags, "Discord_Certified_Moderator")
	}
	if flag&2 != 0 {
		flags = append(flags, "Partnered_Server_Owner")
	}
	if flag&4 != 0 {
		flags = append(flags, "HypeSquad_Events")
	}
	if flag&64 != 0 {
		flags = append(flags, "House_Bravery")
	}
	if flag&128 != 0 {
		flags = append(flags, "House_Brilliance")
	}
	if flag&256 != 0 {
		flags = append(flags, "House_Balance")
	}
	if flag&8 != 0 {
		flags = append(flags, "Bug_Hunter_Level_1")
	}
	if flag&16384 != 0 {
		flags = append(flags, "Bug_Hunter_Level_2")
	}
	if flag&4194304 != 0 {
		flags = append(flags, "Active_Developer")
	}
	if flag&131072 != 0 {
		flags = append(flags, "Early_Verified_Bot_Developer")
	}
	if flag&512 != 0 {
		flags = append(flags, "Early_Supporter")
	}

	return flags
}
