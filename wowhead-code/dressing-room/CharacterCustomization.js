WH.Wow.CharacterCustomization = function(e, t, i, r) {
  const E = this;
  const s = WH.Wow.Race;
  this.NPC_CHOICES_ALLOWED = 1;
  this.NPC_CHOICES_OTHER_ALLOWED = 2;
  this.NPC_CHOICES_DISALLOWED = 0;
  this.REQUIREMENT_TYPE_FLAG_PLAYER_CLASS = 1;
  this.REQUIREMENT_TYPE_FLAG_NPC = 2;
  this.REQUIREMENT_TYPE_FLAG_TRANSMOG = 4;
  this.REQUIREMENT_TYPE_MASK_PLAYER_OR_NPC = this.REQUIREMENT_TYPE_FLAG_PLAYER_CLASS | this.REQUIREMENT_TYPE_FLAG_NPC;
  const _ = e;
  const c = t;
  const n = i;
  const R = r ?? [];
  this.choiceIsNpcOnly = (e, t) => (e.requirementType & E.REQUIREMENT_TYPE_MASK_PLAYER_OR_NPC) === E.REQUIREMENT_TYPE_FLAG_NPC || t === E.NPC_CHOICES_OTHER_ALLOWED && E.choiceRequirementTypeIsOther(e);
  this.choiceRequirementsMet = (e, t, i, r, s, _) => {
      if (_ !== E.NPC_CHOICES_OTHER_ALLOWED) {
          let E = n[e.requirementId];
          if (E) {
              let e = false;
              let _ = u(i, r, s);
              for (let i, r = 0; i = _[r]; r++) {
                  if (E.indexOf(t[i.slug]) >= 0 || t[i.slug] === undefined) {
                      e = true;
                      break
                  }
              }
              if (!e) {
                  return false
              }
          }
      }
      let c = e.requirementType || 0;
      if (c === 0) {
          return true
      }
      let R = !e.requiredRaces || e.requiredRaces.includes(i);
      if ((c & E.REQUIREMENT_TYPE_FLAG_PLAYER_CLASS) !== 0) {
          return R && (e.classMask === 0 || (1 << s - 1 & e.classMask) !== 0)
      }
      return R && !!_ && E.choiceIsNpcOnly(e)
  }
  ;
  this.choiceRequirementTypeIsOther = e => {
      let t = e.requirementType || 0;
      return t !== 0 && (t & E.REQUIREMENT_TYPE_MASK_PLAYER_OR_NPC) === 0
  }
  ;
  this.getRandomChoices = (e, t, i, r, s) => {
      let _ = {};
      let c = u(e, t, i);
      c.sort(( (e, t) => t.choices.length - e.choices.length));
      let R = [];
      c.forEach((c => {
          _[s ? c.id : c.slug] = undefined;
          let u = [];
          let l = false;
          c.choices.forEach((s => {
              if (R.indexOf(s.id) >= 0) {
                  if (!l) {
                      u = [];
                      l = true
                  }
              }
              if ((!l || R.indexOf(s.id) >= 0) && E.choiceRequirementsMet(s, _, e, t, i, r)) {
                  u.push(s)
              }
          }
          ));
          if (u.length) {
              let e = u[Math.floor(Math.random() * u.length)];
              _[s ? c.id : c.slug] = e.id;
              let t = n[e.requirementId];
              if (t) {
                  R = R.concat(t)
              }
          }
      }
      ));
      return _
  }
  ;
  function u(e, t, i) {
      if (!s.isPlayableRace(e)) {
          i = 0
      }
      const r = (c[_[e]?.[t]] || []).slice();
      R.filter((t => {
          let r = t.requirementType || 0;
          if (r === 0) {
              return true
          }
          if (t.requiredRaces && !t.requiredRaces.includes(e)) {
              return false
          }
          if ((r & E.REQUIREMENT_TYPE_FLAG_PLAYER_CLASS) === 0) {
              return true
          }
          return t.classMask === 0 || (1 << i - 1 & t.classMask) !== 0
      }
      )).map((e => (c[e.model] || []).filter((t => t.category = e.category)))).forEach((e => e.forEach((e => r.push(e)))));
      return r
  }
}
;
