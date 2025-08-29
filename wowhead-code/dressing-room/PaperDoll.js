var g_paperdolls = {};
function Paperdoll(e) {
    const t = WH.Wow.Item;
    const i = WH.Wow;
    this.pepe = 0;
    this.race = 1;
    this.gender = 0;
    this.characterModel = null;
    this.charClass = 1;
    this.slotHandedness = {
        15: "main",
        21: "main",
        25: "main",
        28: "main",
        14: "off",
        22: "off",
        23: "off"
    };
    this.forceSD = !WH.Wow.Expansion.available(WH.Wow.Expansion.WOD);
    this.forceHD = WH.Wow.Expansion.available(WH.Wow.Expansion.SL);
    WH.cO(this, e);
    if (!this.id || !this.container) {
        return
    }
    this.container.classList.add("paperdoll");
    this.slots = WH.cOr([], g_character_slots_data);
    if (e.canUseSeparateShoulders) {
        for (let e = 0, s; s = this.slots[e]; e++) {
            if (s.name === "shoulder") {
                this.slots.splice(e + 1, 0, {
                    id: Paperdoll.INVENTORY_SLOT_SHOULDER_2,
                    name: "shoulder",
                    internalSlot: i.INVENTORY_SLOT_SHOULDERS,
                    itemSlots: [t.INVENTORY_TYPE_SHOULDERS]
                });
                break
            }
        }
    }
    this.iconSize = this.iconSize || WH.Icon.MEDIUM;
    this.enableHD = !this.forceSD;
    for (var s in this.slots) {
        this.slots[s].data = {
            raw: [this.slots[s].id, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
            slot: this.slots[s].itemSlots[0]
        }
    }
    if (g_paperdolls[this.id] && g_paperdolls[this.id].viewer) {
        g_paperdolls[this.id].viewer.destroy();
        g_paperdolls[this.id].viewer = null
    }
    g_paperdolls[this.id] = this;
    this.initialize()
}
Paperdoll.CHARACTER_RATIO = 1.314079422382671;
Paperdoll.INVENTORY_SLOT_SHOULDER_2 = 20;
Paperdoll.prototype = {
    initialize: function() {
        const e = this;
        const t = WH.LocalStorage;
        this.container.dataset.iconSize = this.iconSize;
        this.container.dataset.separateShoulders = JSON.stringify(!!this.separateShoulders);
        var i = WH.ce("div", {
            className: "paperdoll-left"
        });
        var s = WH.ce("div", {
            className: "paperdoll-right"
        });
        var a = WH.ce("div", {
            className: "paperdoll-bottom"
        });
        var o = WH.ce("div", {
            className: "paperdoll-model"
        });
        var n = WH.ce("div", {
            className: "paperdoll-model-inner",
            id: "paperdoll-model-" + this.id
        });
        var l = WH.ce("a", {
            className: "paperdoll-model-pause fa fa-pause"
        });
        var r = WH.ce("div", {
            className: "paperdoll-responsive-shell"
        });
        var h = WH.ce("div", {
            className: "paperdoll-responsive-expander"
        });
        WH.ae(this.container, i);
        WH.ae(this.container, s);
        WH.ae(this.container, a);
        WH.ae(o, n);
        WH.ae(o, l);
        WH.ae(this.container, o);
        WH.ae(r, h);
        WH.ae(this.container, r);
        WH.aE(l, "click", (function() {
            if (this.classList.contains("fa-pause")) {
                this.classList.add("fa-play");
                this.classList.remove("fa-pause");
                e.viewer.method("setAnimPaused", true)
            } else {
                this.classList.add("fa-pause");
                this.classList.remove("fa-play");
                e.viewer.method("setAnimPaused", false)
            }
        }
        ));
        WH.Tooltips.attachNonTouch(l, (function() {
            return this.classList.contains("fa-pause") ? WH.TERMS.pause : WH.TERMS.play
        }
        ), "q2");
        this.controlsRight = WH.ce("div", {
            className: "paperdoll-controls-right"
        });
        WH.ae(this.container, this.controlsRight);
        var d = WH.ce("div", {
            className: "paperdoll-fullscreen",
            innerHTML: " " + WH.TERMS.fullscreenmodelviewer_tip
        });
        WH.aef(d, WH.ce("i", {
            className: "fa fa-expand"
        }));
        WH.ae(this.controlsRight, d);
        setTimeout((function() {
            $(d).fadeOut(2e3)
        }
        ), 7e3);
        this.controlsLeft = WH.ce("div", {
            className: "paperdoll-controls-left"
        });
        WH.ae(this.container, this.controlsLeft);
        var c = WH.ce("a");
        c.href = "javascript:";
        $(c).on("click", e.onClickChooseMount.bind(this));
        c.innerHTML = WH.TERMS.choosemount;
        var p = WH.ce("div", {
            className: "paperdoll-mount"
        });
        WH.ae(p, c);
        WH.ae(this.controlsLeft, p);
        var f = WH.ce("select", {
            ariaLabel: WH.TERMS.animation
        });
        WH.aE(f, "change", (function() {
            e.onAnimationChange.call(e)
        }
        ));
        var u = WH.ce("div", {
            className: "paperdoll-animation"
        });
        WH.ae(u, f);
        WH.ae(this.controlsRight, u);
        this.slotsById = {};
        let W;
        let H = this.canUseSeparateShoulders ? 9 : 8;
        let m = this.canUseSeparateShoulders ? 17 : 16;
        for (let e = 0, t; t = this.slots[e]; ++e) {
            let o = "inventoryslot_" + (t.iconName || t.name);
            W = WH.Icon.create(o, this.iconSize, "javascript:");
            W.dataset.characterSlot = t.id;
            W.tabIndex = 0;
            let n = WH.qs("a", W);
            n.tabIndex = -1;
            n.ariaLabel = t.name;
            WH.ae(e < H ? i : e < m ? s : a, W);
            this.slotsById[t.id] = t;
            this.slotsById[t.id].icon = W;
            if (typeof this.onCreateSlot == "function") {
                this.onCreateSlot(this.slotsById[t.id])
            }
        }
        this.enchantSlots = {};
        if (typeof this.onCreateEnchantSlot === "function") {
            let e = {};
            e[WH.Wow.INVENTORY_SLOT_MAIN_HAND] = "beforebegin";
            e[WH.Wow.INVENTORY_SLOT_OFF_HAND] = "afterend";
            for (let t in e) {
                if (!e.hasOwnProperty(t) || !this.slotsById[t]) {
                    continue
                }
                W = WH.Icon.create("inventoryslot_enchant", this.iconSize, "javascript:");
                W.classList.add("paperdoll-enchant");
                W.classList.add("paperdoll-enchant-disabled");
                this.enchantSlots[t] = {
                    slotId: parseInt(t),
                    icon: W
                };
                this.slotsById[t].icon.insertAdjacentElement(e[t], W);
                this.onCreateEnchantSlot(this.enchantSlots[t])
            }
        }
        this.equipList = [];
        if (this.data) {
            for (let e = 0; e < this.data.length; ++e) {
                if (!this.data[e].raw) {
                    continue
                }
                this.setSlot(this.data[e], false)
            }
        }
        if (typeof this.sheathMain != "number") {
            this.sheathMain = -1
        }
        if (typeof this.sheathOff != "number") {
            this.sheathOff = -1
        }
        this.sheathed = t.get(t.KEY_WOW_MODEL_VIEWER_SHEATHED) === true;
        this.sheathedLabel = WH.ce("label", {
            className: "paperdoll-sheathed",
            innerHTML: WH.TERMS.sheathed,
            onmousedown: WH.rf
        });
        WH.aef(this.sheathedLabel, WH.ce("input", {
            type: "checkbox",
            checked: this.sheathed,
            onchange: function() {
                e.sheathed = this.checked;
                t.set(t.KEY_WOW_MODEL_VIEWER_SHEATHED, !!e.sheathed);
                e.setCharAppearance()
            }
        }));
        WH.ae(a, this.sheathedLabel);
        if (this.sheathMain == -1 && this.sheathOff == -1) {
            this.sheathedLabel.style.display = "none"
        }
        if (this.hideSlots && this.hideSlots instanceof Array) {
            for (let e = 0, t; t = this.hideSlots[e]; e++) {
                if (this.slotsById[t]) {
                    this.slotsById[t].icon.classList.add("paperdoll-hidden");
                    this.slotsById[t].icon.tabIndex = -1
                }
            }
        }
        this.paperdollModel = o;
        this.paperdollModelInner = n;
        this.render();
        setTimeout(this.checkModelPosition.bind(this), 1);
        WH.aE(window, "resize", this.checkModelPosition.bind(this))
    },
    canEnchantSlot: function(e) {
        if (!this.enchantSlots[e]) {
            return false
        }
        let t = this.slotsById[e].data && this.slotsById[e].data.raw && this.slotsById[e].data.raw.length > 1 ? this.slotsById[e].data.raw[1] : 0;
        if (!t) {
            return false
        }
        let i = WH.Gatherer.get(WH.Types.ITEM, t);
        if (!i || (i.classs || i["class"]) !== WH.Wow.Item.CLASS_WEAPON) {
            return false
        }
        let s = [WH.Wow.Item.WEAPON_SUBCLASS_BOW, WH.Wow.Item.WEAPON_SUBCLASS_GUN, WH.Wow.Item.WEAPON_SUBCLASS_THROWN, WH.Wow.Item.WEAPON_SUBCLASS_CROSSBOW, WH.Wow.Item.WEAPON_SUBCLASS_WAND, WH.Wow.Item.WEAPON_SUBCLASS_FISHING_POLE];
        if (s.indexOf(i.subclass) >= 0) {
            return false
        }
        return true
    },
    checkModelPosition: function() {
        if (this.paperdollModel.offsetWidth + 94 > this.container.offsetWidth) {
            this.paperdollModel.style.left = Math.floor((this.container.offsetWidth - this.paperdollModel.offsetWidth) / 2) + "px";
            this.modelPositionModified = true
        } else if (this.modelPositionModified) {
            this.paperdollModel.style.left = ""
        }
    },
    getInvTypeBySlot: function(e) {
        const t = WH.Wow.Item;
        const i = WH.Wow;
        switch (e) {
        case i.INVENTORY_SLOT_MAIN_HAND:
            return t.INVENTORY_TYPE_MAIN_HAND;
        case i.INVENTORY_SLOT_OFF_HAND:
            return t.INVENTORY_TYPE_OFF_HAND;
        case i.INVENTORY_SLOT_CHEST:
            return t.INVENTORY_TYPE_CHEST
        }
        return WH.Wow.Item.getSlotsToInvTypes()[e][0]
    },
    getPepeDisplayId: function() {
        if (!WH.Wow.Models.isPepeEnabled() || isNaN(this.pepe) || this.pepe <= 0) {
            return 0
        }
        return 999989 + this.pepe
    },
    getShouldersOverrideData: function() {
        const e = WH.Wow;
        let t = false;
        let i = [0, 0];
        for (let s, a = 0; s = this.equipList[a]; a++) {
            if (s.characterSlot === e.INVENTORY_SLOT_SHOULDERS) {
                i[0] = s.display;
                if (s.hasOwnProperty("display2")) {
                    i = [s.display2, s.display];
                    t = true;
                    break
                }
            } else if (s.characterSlot === Paperdoll.INVENTORY_SLOT_SHOULDER_2) {
                i[1] = s.display
            }
        }
        if (!this.separateShoulders && !t) {
            i[1] = i[0]
        }
        i.reverse();
        return i
    },
    getViewerOptions: function(e) {
        let t = {
            type: e.WOW,
            contentPath: WH.Wow.ModelViewer.getContentPath(WH.getDataEnv()),
            container: $(this.paperdollModelInner),
            aspect: this.paperdollModelInner.offsetWidth / this.paperdollModelInner.offsetHeight,
            background: this.background || WH.Wow.ModelViewer.getBackgroundFilename()
        };
        if (this.modelWheelEventValidation) {
            t.wheelEventValidation = this.modelWheelEventValidation
        }
        if (this.displayId) {
            t.models = {
                id: this.displayId,
                type: e.Wow.Types.NPC
            };
            return t
        }
        let i = [];
        let s = this.equipList;
        for (let e, t = 0; e = s[t]; t++) {
            let t = [e.invSlot, e.display];
            if (e.enchantVisual) {
                t.push(e.enchantVisual)
            }
            i.push(t)
        }
        let a = this.getPepeDisplayId();
        if (a) {
            i.push([27, a])
        }
        let o = this.updateAppearanceData({
            hairstyle: this.hairstyle,
            haircolor: this.haircolor,
            facetype: this.facetype,
            skincolor: this.skincolor,
            features: this.features,
            hornstyle: this.hornstyle,
            blindfolds: this.blindfolds,
            tattoos: this.tattoos,
            sheathMain: this.sheathMain,
            sheathOff: this.sheathOff
        });
        t.cls = this.charClass;
        t.items = i;
        if (this.canUseSeparateShoulders) {
            t.shouldersOverride = this.getShouldersOverrideData()
        }
        t.mount = {
            id: this.npcModel,
            type: e.Wow.Types.NPC
        };
        t.models = {
            id: this.characterModel ?? WH.Wow.ModelViewer.getModelId(this.race, this.gender),
            type: e.Wow.Types.CHARACTER
        };
        let n = this.sheathed ? o.sheathMain : -1;
        let l = this.sheathed ? o.sheathOff : -1;
        if (this.charCustomization) {
            this.charCustomization.sheathMain = n;
            this.charCustomization.sheathOff = l;
            t.charCustomization = this.charCustomization
        } else {
            WH.cO(t, {
                sk: this.skincolor,
                ha: this.hairstyle,
                hc: this.haircolor,
                fa: this.facetype,
                fh: this.features,
                fc: this.haircolor,
                ep: this.blindfolds,
                ho: this.hornstyle,
                ta: this.tattoos,
                sheathMain: n,
                sheathOff: l
            })
        }
        if (this.forceSD) {
            t.hd = false
        } else if (this.forceHD) {
            t.hd = true
        } else {
            t.hd = this.enableHD;
            if (this.race == 10) {
                t.hd = true
            }
        }
        if (this.characterModel) {
            t.items = [];
            this.charCustomization.sheathMain = -1;
            this.charCustomization.sheathOff = -1;
            t.shouldersOverride = [0, 0]
        }
        return t
    },
    setPepe: function(e) {
        if (!WH.Wow.Models.isPepeEnabled() || this.pepe === e) {
            return
        }
        this.pepe = e;
        WH.debug("Setting Pepe to version", e, "with display ID", this.getPepeDisplayId());
        this.updateViewer(27, this.getPepeDisplayId())
    },
    setSlot: function(e, t) {
        const i = WH.Wow.Item;
        const s = WH.Wow;
        if (e && e.raw && e.raw.length > 0) {
            var a = e.raw;
            var o = a[0];
            if (o && this.slotsById[o]) {
                var n = a[1];
                var l = a[9];
                var r = a[10];
                var h = a.slice(WH.Paperdoll.BONUS_INDEX_START, WH.Paperdoll.BONUS_INDEX_START + WH.Paperdoll.EQUIPSET_MAX_BONUSES);
                let T = a[30];
                let N = a[31];
                var d = this.slotsById[o].data.raw && this.slotsById[o].data.raw.length > 1 ? this.slotsById[o].data.raw[1] : 0;
                this.slotsById[o].data.raw = a;
                var c = this.slotsById[o].icon;
                var p = this.slotsById[o].invSlot ? this.slotsById[o].invSlot : null;
                var f = -1;
                if (t) {
                    for (var u = 0; u < this.equipList.length; u++) {
                        if (this.equipList[u].characterSlot === o) {
                            f = u;
                            break
                        }
                    }
                }
                if (this.enchantSlots[o]) {
                    WH.Icon.setName(this.enchantSlots[o].icon, this.iconSize, e.enchantVisual ? e.enchantIcon || "inv_scroll_05" : "inventoryslot_enchant");
                    if (this.canEnchantSlot(o)) {
                        this.enchantSlots[o].icon.classList.remove("paperdoll-enchant-disabled");
                        this.enchantSlots[o].icon.tabIndex = 0
                    } else {
                        this.enchantSlots[o].icon.classList.add("paperdoll-enchant-disabled")
                    }
                }
                let g = WH.Gatherer.get(WH.Types.ITEM, n);
                if (g) {
                    let d = g.quality === WH.Wow.Item.QUALITY_ARTIFACT ? e.artifactAppearanceMod : null;
                    WH.Icon.setName(c, this.iconSize, g_items.getIcon(n, h, d));
                    let u = WH.Icon.getLink(c);
                    var W = [];
                    for (var H in h) {
                        if (h[H]) {
                            W.push(h[H])
                        }
                    }
                    let w = {};
                    if (W.length) {
                        w.bonus = W.join(":")
                    }
                    if (e.level) {
                        var m = WH.applyStatModifications(e.jsonequip, null, e.upgrade.step, null, e.raw.slice(WH.Paperdoll.BONUS_INDEX_START, WH.Paperdoll.BONUS_INDEX_START + WH.Paperdoll.EQUIPSET_MAX_BONUSES), e.raw[WH.Paperdoll.TIMEWALKER_LEVEL_INDEX]);
                        w.ilvl = m.level
                    }
                    u.href = WH.Entity.getUrl(WH.Types.ITEM, n, null, w);
                    u.rel = this.getItemRel(a);
                    u.target = "_blank";
                    let _ = g.jsonequip;
                    let y = WH.Gatherer.get(WH.Types.ITEM, l);
                    if (y) {
                        _ = y.jsonequip
                    }
                    let O = [s.INVENTORY_SLOT_MAIN_HAND, s.INVENTORY_SLOT_OFF_HAND].includes(o) && ![i.INVENTORY_TYPE_SHIELD, i.INVENTORY_TYPE_RANGED].includes(_.slotbak) ? this.getInvTypeBySlot(o) : _.slotbak;
                    var S = WH.Wow.ModelViewer.getEffectiveInventoryType(O);
                    if (this.slotHandedness[S] && this.slotHandedness[this.slotsById[o].translatedSlot] && this.slotHandedness[S] != this.slotHandedness[this.slotsById[o].translatedSlot]) {
                        this.updateViewer(this.slotsById[o].translatedSlot || ([i.INVENTORY_TYPE_SHIELD, i.INVENTORY_TYPE_RANGED].includes(this.slotsById[o].slotbak) ? this.getInvTypeBySlot(o) : p))
                    }
                    var v = _.displayid;
                    var I = g_items.getAppearance(l ? l : n, l ? [] : h, d);
                    if (I != null && I[0]) {
                        v = I[0]
                    }
                    v = T || v;
                    var E = undefined;
                    if (e.enchantVisual && WH.Wow.ModelViewer.supportsEnchantVisual(O) && this.canEnchantSlot(o)) {
                        E = e.enchantVisual
                    }
                    if (v && !this.slotsById[o].noModel && !r && n != 5976 && n != 69209 && n != 69210) {
                        let e = {
                            characterSlot: o,
                            display: v,
                            enchantVisual: E,
                            invSlot: O
                        };
                        if (N != null) {
                            e.display2 = N
                        }
                        if (f == -1) {
                            this.equipList.push(e)
                        } else {
                            this.equipList.splice(f, 1, e)
                        }
                        if (t) {
                            this.updateViewer(O, v, E)
                        }
                    }
                    this.slotsById[o].invSlot = O;
                    this.slotsById[o].slotbak = _.slotbak;
                    this.slotsById[o].translatedSlot = S
                } else {
                    if (t && p) {
                        WH.Icon.setName(c, this.iconSize, "inventoryslot_" + this.slotsById[o].name);
                        let e = WH.Icon.getLink(c);
                        e.href = "javascript:";
                        e.rel = "";
                        if (f != -1) {
                            this.equipList.splice(f, 1)
                        }
                        this.updateViewer(this.slotsById[o].translatedSlot || ([i.INVENTORY_TYPE_SHIELD, i.INVENTORY_TYPE_RANGED].includes(this.slotsById[o].slotbak) ? this.getInvTypeBySlot(o) : p))
                    }
                }
                var w = [d, n];
                for (var _ = 0; _ < 2; ++_) {
                    var y = w[_];
                    if (y && g_items[y] && g_items[y].jsonequip.itemset) {
                        for (var u in this.slotsById) {
                            if (this.slotsById[u].data.raw && this.slotsById[u].data.raw.length > 1) {
                                WH.Icon.getLink(this.slotsById[u].icon).rel = this.getItemRel(this.slotsById[u].data.raw)
                            }
                        }
                    }
                }
                if (typeof this.onCreateSlot == "function") {
                    this.onCreateSlot(this.slotsById[o])
                }
            }
        }
    },
    setCharAppearance: function(e) {
        e = this.updateAppearanceData(e || {});
        let t = this.sheathed ? e.sheathMain : -1;
        let i = this.sheathed ? e.sheathOff : -1;
        if (e.charCustomization) {
            WH.debug("Setting model viewer appearance:", e.charCustomization);
            this.viewer.method("setAppearance", e.charCustomization)
        } else {
            WH.debug("Setting legacy model viewer appearance:", "hairstyle", e.hairstyle, "haircolor", e.haircolor, "facetype", e.facetype, "skincolor", e.skincolor, "features", e.features, "haircolor", e.haircolor, "hornstyle", e.hornstyle, "blindfolds", e.blindfolds, "tattoos", e.tattoos, "sheathMain", t, "sheathOffhand", i);
            this.viewer.method("setAppearance", [e.hairstyle, e.haircolor, e.facetype, e.skincolor, e.features, e.haircolor, e.hornstyle, e.blindfolds, e.tattoos, t, i])
        }
        WH.debug("Setting sheath types:", [t, i]);
        this.viewer.method("setSheath", [t, i])
    },
    setOffset: function(e) {
        this.viewer.setOffset.apply(this.viewer, e)
    },
    setSeparateShoulders: function(e) {
        e = !!e;
        if (this.separateShoulders === e) {
            return
        }
        this.separateShoulders = e;
        if (this.container) {
            this.container.dataset.separateShoulders = JSON.stringify(this.separateShoulders)
        }
        if (this.viewer) {
            this.updateShoulders()
        }
    },
    setZoom: function(e) {
        this.viewer.setZoom(e / 10)
    },
    updateAppearanceData: function(e) {
        if (this.lastAppearanceData) {
            e = WH.cO(this.lastAppearanceData, e)
        }
        if (typeof e.sheathMain !== "number") {
            e.sheathMain = -1
        }
        if (typeof e.sheathOff !== "number") {
            e.sheathOff = -1
        }
        this.lastAppearanceData = e;
        return e
    },
    updateCharAppearance: function(e) {
        if (this.viewer != null) {
            this.setCharAppearance(e)
        }
    },
    updateShoulders: function() {
        this.viewer.method("setShouldersOverride", [this.getShouldersOverrideData()])
    },
    updateSlots: function(e) {
        if (!this.viewer) {
            return
        }
        for (let t = 0, i; i = e[t]; t++) {
            this.setSlot(i, true)
        }
    },
    updateViewer: function(e, t, i) {
        const s = WH.Wow.Item;
        if (e === s.INVENTORY_TYPE_SHOULDERS && this.canUseSeparateShoulders) {
            this.updateShoulders();
            return
        }
        let a = e;
        if (a === s.INVENTORY_TYPE_ROBE) {
            a = s.INVENTORY_TYPE_CHEST
        }
        WH.debug("Clearing model viewer slot:", a.toString());
        this.viewer.method("clearSlots", a.toString());
        if (t) {
            WH.debug("Attaching to model viewer slot:", e.toString(), "Display ID:", t, "Enchant Visual:", i);
            this.viewer.method("setItems", [[{
                slot: e,
                display: t,
                visual: i || 0
            }]])
        }
    },
    render: function() {
        if (!WH.Wow.ModelViewer.isLibraryLoaded()) {
            WH.Wow.ModelViewer.loadLibrary(this.render.bind(this));
            return
        }
        var e = WH.Wow.ModelViewer.getLibrary();
        if (this.viewer)
            this.viewer.destroy();
        let t = this.getViewerOptions(e);
        if (WH.Wow.ModelViewer.canTakeScreenshots() && WH.getGets().modelViewerTransparency !== undefined) {
            t.transparent = true;
            delete t.background;
            let e = WH.Wow.ModelViewer.getBackgroundUrl();
            this.paperdollModelInner.backgroundImage = 'url("' + e + '")'
        }
        WH.Wow.ModelViewer.applyDataEnv(t, WH.getDataEnv());
        WH.debug("Creating model viewer with these options:", t);
        this.viewer = new e(t);
        if (this.resizable != null) {
            this.viewer.setAdaptiveMode(this.resizable)
        }
        if (this.zoom != null) {
            this.setZoom(this.zoom)
        }
        if (this.offset != null) {
            this.setOffset(this.offset)
        }
        this.loadAnimations()
    },
    onSelectMount: function(e) {
        Lightbox.hide();
        this.npcModel = e.npcmodel ? e.npcmodel : 0;
        if (typeof this.onChangeMount == "function") {
            this.onChangeMount(this.npcModel)
        }
        this.render()
    },
    onChooseMountPicker: function(e, t, i) {
        Lightbox.setSize(800);
        if (t) {
            e.className += " paperdoll-picker";
            var s = WH.ce("div");
            s.className = "lightbox-content listview";
            WH.ae(e, s);
            var a = WH.ce("a");
            a.className = "dialog-option fa fa-times";
            a.href = "javascript:";
            a.onclick = Lightbox.hide;
            WH.ae(a, WH.ct(WH.TERMS.close));
            WH.ae(e, a);
            this.mountLv = new Listview({
                template: "mountsgallery",
                id: "mountsgallery",
                containVerticalScrolling: true,
                data: [],
                selectData: this.onSelectMount.bind(this),
                hideCount: 1,
                parent: s,
                hideBands: 2,
                hideNav: 1 | 2,
                hideHeader: 1,
                searchable: 1,
                hash: Listview.HASH_DISABLED,
                filtrable: 0,
                forceBandTop: 1,
                clip: {
                    w: 780,
                    h: 486
                }
            })
        }
        setTimeout(function() {
            this.mountLv.Filters.focus();
            WH.displayNone(this.mountLv.noteTop)
        }
        .bind(this), 10);
        if (!WH.isSet("g_mounts")) {
            this.loadMountData()
        }
    },
    loadMountData: function() {
        const e = this;
        $.getScript(WH.Url.getDataPageUrl("mount"), (function() {
            if (!WH.isSet("g_mounts")) {
                return
            }
            var t = [{
                none: 1
            }];
            var i = $.map(g_mounts, (function(e) {
                return [e]
            }
            ));
            e.mountLv.setData(t.concat(i));
            e.mountLv.refreshRows(true)
        }
        ))
    },
    onClickChooseMount: function() {
        Lightbox.show("mountpicker", {
            onShow: this.onChooseMountPicker.bind(this)
        })
    },
    resetPosition: function() {
        this.viewer.renderer.azimuth = Math.PI * 1.5;
        this.viewer.renderer.zenith = Math.PI / 2;
        this.viewer.renderer.translation = [0, 0, 0];
        this.viewer.renderer.zoom.target = 1;
        WH.debug("Resetting the model viewer animation to:", WH.Wow.ModelViewer.ANIMATION_CHARACTER_CREATE);
        this.viewer.method("setAnimation", WH.Wow.ModelViewer.ANIMATION_CHARACTER_CREATE)
    },
    getItemRel: function(e) {
        if (!g_items[e[1]])
            return;
        var t = WH.applyStatModifications(g_items[e[1]].jsonequip, e[2], e[8], 0, e.slice(WH.Paperdoll.BONUS_INDEX_START, WH.Paperdoll.BONUS_INDEX_START + WH.Paperdoll.EQUIPSET_MAX_BONUSES), e[WH.Paperdoll.TIMEWALKER_LEVEL_INDEX]);
        var i = e[4 + (t.nsockets | 0)] ? 1 : 0
          , s = []
          , a = []
          , o = [];
        if (e[2])
            s.push("rand=" + e[2]);
        if (e[3])
            s.push("ench=" + e[3]);
        for (var n = 0, l = (t.nsockets | 0) + i; n < l; ++n)
            a.push(e[4 + n] > 0 ? e[4 + n] : 0);
        if (a.length)
            s.push("gems=" + a.join(":"));
        var r = e.slice(WH.Paperdoll.BONUS_INDEX_START, WH.Paperdoll.BONUS_INDEX_START + WH.Paperdoll.EQUIPSET_MAX_BONUSES);
        var h = [];
        for (var n = 0; n < r.length; ++n) {
            if (r[n]) {
                h.push(r[n])
            }
        }
        if (h.length > 0) {
            h.sort();
            s.push("bonus=" + h.join(":"))
        }
        if (i)
            s.push("sock");
        if (g_items[e[1]].jsonequip.itemset) {
            for (var n in this.slotsById) {
                var d = this.slotsById[n].data.raw ? this.slotsById[n].data.raw[1] : 0;
                if (d && g_items[d] && g_items[d].jsonequip.itemset)
                    o.push(d)
            }
            s.push("pcs=" + o.join(":"))
        }
        if (this.level < 90)
            s.push("lvl=" + this.level);
        var c = s.join("&");
        if (c)
            c = "&" + c;
        return c
    },
    onAnimationChange: function() {
        var e = $(".paperdoll-animation select", this.container);
        if (this.viewer && this.viewer.method("isLoaded") && e.val()) {
            var t = e.val();
            WH.debug("Setting model viewer animation to:", t);
            this.viewer.method("setAnimation", t)
        }
    },
    loadAnimations: function() {
        const e = this;
        var t = window.setTimeout((function() {
            e.loadAnimations.call(e)
        }
        ), 500);
        if (!e.hasOwnProperty("animsLoaded"))
            e.animsLoaded = false;
        if (e.animsLoaded) {
            if (this.viewer && this.viewer.method("isLoaded")) {
                this.viewer.method("setAnimation", WH.Wow.ModelViewer.ANIMATION_CHARACTER_CREATE)
            }
            return
        }
        if (!this.viewer || !this.viewer.method("isLoaded")) {
            return
        }
        window.clearTimeout(t);
        WH.displayBlock(".paperdoll-mount", e.container);
        if (this.animationControlHandler) {
            this.animationControlHandler(this.viewer);
            return
        }
        var i = $(".paperdoll-animation select", e.container);
        i.empty();
        i.parent().show();
        var s = {};
        var a = this.viewer.method("getNumAnimations");
        for (var o = 0; o < a; ++o) {
            let e = this.viewer.method("getAnimation", o);
            if (e) {
                s[e] = 1
            }
        }
        var n = [];
        for (var l in s)
            n.push(l);
        n.sort();
        i.append($("<option/>", {
            text: WH.TERMS.animation,
            disabled: true,
            selected: true
        }));
        for (var o = 0; o < n.length; ++o)
            i.append($("<option/>", {
                text: n[o],
                val: n[o]
            }));
        this.viewer.method("setAnimation", WH.Wow.ModelViewer.ANIMATION_CHARACTER_CREATE);
        e.animsLoaded = true
    }
};
Listview.templates.mountsgallery = {
    sort: [1],
    mode: Listview.MODE_TILED,
    nItemsPerPage: -1,
    hash: Listview.HASH_DISABLED,
    columns: [{
        id: "name",
        name: WH.TERMS.name,
        value: "name",
        sortFunc: this.sortFunc,
        type: Listview.COLUMN_TYPE_TEXT
    }],
    compute: function(e, t, i) {
        const s = this;
        WH.aE(t, "click", (function() {
            s.selectData(e)
        }
        ));
        t.className = "screenshot-cell";
        t.vAlign = "bottom";
        if (e.npcmodel) {
            let i = WH.ce("img", {
                src: WH.Wow.Npc.getThumbUrl(e.npcmodel)
            });
            WH.ae(t, WH.ce("a", {
                href: "javascript:",
                rel: "spell=" + e.id
            }, i))
        }
        let a = WH.ce("div", {
            className: "screenshot-caption"
        });
        let o = WH.ce("a");
        if (e.none) {
            o.classList.add("q0");
            WH.st(o, WH.TERMS.none)
        } else {
            o.classList.add("q");
            o.href = WH.Entity.getUrl(WH.Types.SPELL, e.id, e.name);
            WH.st(o, e.name)
        }
        WH.ae(a, o);
        let n = WH.ce("div", {
            className: "screenshot-caption-wrapper"
        });
        WH.ae(n, a);
        WH.ae(t, n)
    },
    sortFunc: function(e, t) {
        return WH.stringCompare(e.displayid, t.displayid) || WH.stringCompare(e.name, t.name)
    }
};
