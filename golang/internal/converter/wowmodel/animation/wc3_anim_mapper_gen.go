package animation

// Code generated from animation-mapper.ts; DO NOT EDIT.

func getWc3AnimName(wowAnimName string) wc3AnimInfo {
	loopTrue := true
	switch wowAnimName {
	case "Stand":
		return wc3AnimInfo{wc3Name: "Stand", loop: &loopTrue}
	case "Death":
		return wc3AnimInfo{wc3Name: "Death"}
	case "Spell":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "Dead":
		return wc3AnimInfo{wc3Name: "Decay"}
	case "Rise":
		return wc3AnimInfo{wc3Name: "Birth"}
	case "Birth":
		return wc3AnimInfo{wc3Name: "Birth"}
	case "Sleep":
		return wc3AnimInfo{wc3Name: "Sleep", loop: &loopTrue}
	case "Whirlwind":
		return wc3AnimInfo{wc3Name: "Attack Walk Stand Spin", loop: &loopTrue}
	case "EmoteTalk":
		return wc3AnimInfo{wc3Name: "Portrait Talk", loop: &loopTrue}
	case "EmoteTalkExclamation":
		return wc3AnimInfo{wc3Name: "Portrait Talk", loop: &loopTrue}
	case "EmoteTalkQuestion":
		return wc3AnimInfo{wc3Name: "Portrait Talk", loop: &loopTrue}
	case "EmoteTalkSubdued":
		return wc3AnimInfo{wc3Name: "Portrait Talk", loop: &loopTrue}
	case "EmoteTalkFrustrated":
		return wc3AnimInfo{wc3Name: "Portrait Talk", loop: &loopTrue}
	case "Walk":
		return wc3AnimInfo{wc3Name: "Cinematic Walk", loop: &loopTrue}
	case "Run":
		return wc3AnimInfo{wc3Name: "Walk", loop: &loopTrue}
	case "Sprint":
		return wc3AnimInfo{wc3Name: "Walk Fast", loop: &loopTrue}
	case "SwimIdle":
		return wc3AnimInfo{wc3Name: "Stand Swim", loop: &loopTrue}
	case "MountSwimIdle":
		return wc3AnimInfo{wc3Name: "Stand Swim", loop: &loopTrue}
	case "Swim":
		return wc3AnimInfo{wc3Name: "Walk Swim", loop: &loopTrue}
	case "Drown":
		return wc3AnimInfo{wc3Name: "Death Swim"}
	case "Drowned":
		return wc3AnimInfo{wc3Name: "Decay Swim"}
	case "ReadyUnarmed":
		return wc3AnimInfo{wc3Name: "Stand Ready", attackTag: "Unarmed", loop: &loopTrue}
	case "AttackUnarmed":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "Unarmed"}
	case "AttackFist1H":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "Unarmed"}
	case "AttackFist1HOff":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "Unarmed"}
	case "AttackUnarmedOff":
		return wc3AnimInfo{wc3Name: "Attack ", attackTag: "Unarmed"}
	case "SpecialUnarmed":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "Unarmed"}
	case "Kick":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "Unarmed"}
	case "SwimAttackUnarmed":
		return wc3AnimInfo{wc3Name: "Attack Swim", attackTag: "Unarmed"}
	case "Ready1H":
		return wc3AnimInfo{wc3Name: "Stand Ready", attackTag: "1H", loop: &loopTrue}
	case "Attack1H":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "1H"}
	case "AttackFL":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "1H"}
	case "AttackFLOff":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "1H"}
	case "Attack1HPierce":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "1H"}
	case "AttackOff":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "1H"}
	case "AttackOffPierce":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "1H"}
	case "CombatAbility1HPierce":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "1H"}
	case "CombatAbility1HOffPierce":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "1H"}
	case "Special1H":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "CombatAbility1H01":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "CombatAbility1H01Off":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "CombatAbility1H02":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "CombatAbility1H02Off":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "CombatAbility1H03Off":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "CombatAbility1HBig01":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "CombatBladeStorm":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "SpecialDual":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "CombatAbilityDualWield01":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "1H"}
	case "Ready2H":
		return wc3AnimInfo{wc3Name: "Stand Ready", attackTag: "2H", loop: &loopTrue}
	case "Attack2H":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "2H"}
	case "Attack2HLoosePierce":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "2H"}
	case "Special2H":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "2H"}
	case "CombatAbility2H01":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "2H"}
	case "CombatAbility2H02":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "2H"}
	case "CombatAbility2H03":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "2H"}
	case "CombatAbility2HBig01":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "2H"}
	case "CombatAbility2HBig02":
		return wc3AnimInfo{wc3Name: "Attack Slam", attackTag: "2H"}
	case "Attack2HL":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "2HL"}
	case "Ready2HL":
		return wc3AnimInfo{wc3Name: "Stand Ready", attackTag: "2HL", loop: &loopTrue}
	case "ReadyBow":
		return wc3AnimInfo{wc3Name: "Stand Ready", attackTag: "Bow", loop: &loopTrue}
	case "LoadBow":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "Bow"}
	case "AttackBow":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "Bow"}
	case "ReadyRifle":
		return wc3AnimInfo{wc3Name: "Stand Ready", attackTag: "Rifle", loop: &loopTrue}
	case "LoadRifle":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "Rifle"}
	case "AttackRifle":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "Rifle"}
	case "AttackThrown":
		return wc3AnimInfo{wc3Name: "Attack", attackTag: "Thrown"}
	case "SpellPrecast":
		return wc3AnimInfo{wc3Name: "Spell Channel", loop: &loopTrue}
	case "SpellCast":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "SpellCastArea":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "SpellCastOmni":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "CastCurseRight":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "CastOutStrong":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "CastStrongLeft":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "CastStrongRight":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "CastStrongUpLeft":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "CastStrongUpRight":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "CastSweepRight":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "CastTwistUpBoth":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "ChannelCastOmniUp":
		return wc3AnimInfo{wc3Name: "Spell"}
	case "ReadySpellDirected":
		return wc3AnimInfo{wc3Name: "Stand Channel", loop: &loopTrue}
	case "ReadySpellOmni":
		return wc3AnimInfo{wc3Name: "Stand Channel", loop: &loopTrue}
	case "SpellCastDirected":
		return wc3AnimInfo{wc3Name: "Spell Throw"}
	case "ChannelCastDirected":
		return wc3AnimInfo{wc3Name: "Spell Channel Throw", loop: &loopTrue}
	case "ChannelCastOmni":
		return wc3AnimInfo{wc3Name: "Spell Channel", loop: &loopTrue}
	case "Cannibalize":
		return wc3AnimInfo{wc3Name: "Spell Channel", loop: &loopTrue}
	case "BattleRoar":
		return wc3AnimInfo{wc3Name: "Spell Slam"}
	case "DragonStomp":
		return wc3AnimInfo{wc3Name: "Spell Slam"}
	case "Close":
		return wc3AnimInfo{wc3Name: "Birth"}
	case "Closed":
		return wc3AnimInfo{wc3Name: "Stand", loop: &loopTrue}
	case "Open":
		return wc3AnimInfo{wc3Name: "Death"}
	case "Opened":
		return wc3AnimInfo{wc3Name: "Decay Flesh"}
	case "FlyStand":
		return wc3AnimInfo{wc3Name: "Stand Alternate", loop: &loopTrue}
	case "Hover":
		return wc3AnimInfo{wc3Name: "Stand Alternate", loop: &loopTrue}
	case "MountFlightIdle":
		return wc3AnimInfo{wc3Name: "Stand Alternate", loop: &loopTrue}
	case "Fly":
		return wc3AnimInfo{wc3Name: "Walk Alternate", loop: &loopTrue}
	case "FlyFly":
		return wc3AnimInfo{wc3Name: "Walk Alternate", loop: &loopTrue}
	case "FlyWalk":
		return wc3AnimInfo{wc3Name: "Walk Alternate", loop: &loopTrue}
	case "MountFlightRun":
		return wc3AnimInfo{wc3Name: "Walk Alternate", loop: &loopTrue}
	case "Land":
		return wc3AnimInfo{wc3Name: "Morph Alternate"}
	case "Settle":
		return wc3AnimInfo{wc3Name: "Morph Alternate"}
	case "MountFlightLand":
		return wc3AnimInfo{wc3Name: "Morph Alternate"}
	case "LiftOff":
		return wc3AnimInfo{wc3Name: "Morph"}
	case "MountFlightStart":
		return wc3AnimInfo{wc3Name: "Morph"}
	case "FlyChannelCastDirected":
		return wc3AnimInfo{wc3Name: "Spell Channel Alternate Throw", loop: &loopTrue}
	case "FlyChannelCastOmni":
		return wc3AnimInfo{wc3Name: "Spell Channel Alternate", loop: &loopTrue}
	case "FlySpellCastDirected":
		return wc3AnimInfo{wc3Name: "Spell Alternate Throw"}
	case "FlySpellCastOmni":
		return wc3AnimInfo{wc3Name: "Spell Alternate"}
	case "FlyAttackUnarmed":
		return wc3AnimInfo{wc3Name: "Attack Alternate"}
	case "FlyDragonSpit":
		return wc3AnimInfo{wc3Name: "Attack Alternate"}
	case "DragonSpitHover":
		return wc3AnimInfo{wc3Name: "Attack Alternate"}
	case "FlyDeathEnd":
		return wc3AnimInfo{wc3Name: "Death Alternate"}
	case "AttackJoust":
		return wc3AnimInfo{wc3Name: "Mount AttackJoust"}
	case "HoldJoust":
		return wc3AnimInfo{wc3Name: "Mount HoldJoust", loop: &loopTrue}
	case "LoadJoust":
		return wc3AnimInfo{wc3Name: "Mount LoadJoust"}
	case "ReadyJoust":
		return wc3AnimInfo{wc3Name: "Mount ReadyJoust", loop: &loopTrue}
	case "Mount":
		return wc3AnimInfo{wc3Name: "Mount", loop: &loopTrue}
	case "MountChopper":
		return wc3AnimInfo{wc3Name: "Mount MountChopper", loop: &loopTrue}
	case "MountCrouch":
		return wc3AnimInfo{wc3Name: "Mount MountCrouch", loop: &loopTrue}
	case "MountWide":
		return wc3AnimInfo{wc3Name: "Mount MountWide", loop: &loopTrue}
	case "ReclinedMount":
		return wc3AnimInfo{wc3Name: "Mount ReclinedMount", loop: &loopTrue}
	case "ReclinedMountPassenger":
		return wc3AnimInfo{wc3Name: "Mount ReclinedMountPassenger", loop: &loopTrue}
	case "MountSpecial":
		return wc3AnimInfo{wc3Name: "Stand Victory", loop: &loopTrue}
	case "FlyMountSpecial":
		return wc3AnimInfo{wc3Name: "Stand Alternate Victory", loop: &loopTrue}
	case "Hold":
		return wc3AnimInfo{wc3Name: "Stand", loop: &loopTrue}
	case "Decay":
		return wc3AnimInfo{wc3Name: "Death"}
	case "Spawn":
		return wc3AnimInfo{wc3Name: "Birth"}
	case "Despawn":
		return wc3AnimInfo{wc3Name: "Death"}
	default:
		return wc3AnimInfo{wc3Name: "Cinematic " + wowAnimName}
	}
}

var cinematicLoopWowAnimNames = map[string]struct{}{
	"Stand": {},
	"FlyStand": {},
	"FlyWalk": {},
	"Walk": {},
	"Run": {},
	"Sprint": {},
	"ReadyUnarmed": {},
	"Ready1H": {},
	"Ready2H": {},
	"Ready2HL": {},
	"ReadyBow": {},
	"ReadyRifle": {},
	"ReadyThrown": {},
	"KneelLoop": {},
	"SpellKneelLoop": {},
	"Drowned": {},
	"SwimIdle": {},
	"Walkbackwards": {},
	"Mount": {},
	"Hover": {},
	"Cower": {},
	"Stun": {},
	"EmoteStunNoSheathe": {},
	"Fall": {},
	"Swim": {},
	"SwimLeft": {},
	"SwimRight": {},
	"HoldRifle": {},
	"HoldBow": {},
	"ReadySpellDirected": {},
	"ReadySpellOmni": {},
	"Sleep": {},
	"Jump": {},
	"ShuffleLeft": {},
	"ShuffleRight": {},
	"SwimBackwards": {},
	"ChannelCastDirected": {},
	"ChannelCastOmni": {},
	"Whirlwind": {},
	"EmoteTalk": {},
	"EmoteTalkExclamation": {},
	"EmoteTalkQuestion": {},
	"EmoteTalkSubdued": {},
	"EmoteTalkFrustrated": {},
	"Hold": {},
	"Strangulate": {},
}
