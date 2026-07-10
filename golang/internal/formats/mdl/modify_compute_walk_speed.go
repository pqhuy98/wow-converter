package mdl

func (mod *Modify) ComputeWalkMovespeed() *Modify {
	for i := range mod.MDL.Sequences {
		seq := &mod.MDL.Sequences[i]
		if seq.MoveSpeed != 0 {
			continue
		}
		switch seq.Data.WowName {
		case string(WowAnimWalk):
			seq.MoveSpeed = 2.5
		case string(WowAnimRun):
			seq.MoveSpeed = 7
		case string(WowAnimSprint):
			seq.MoveSpeed = 11.9
		}
	}
	return mod
}
