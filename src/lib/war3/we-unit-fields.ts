// Auto-generated from unitmetadata.slk + WorldEditStrings.txt
// Regenerate: python scripts/gen-we-unit-fields.py

export const WE_UNIT_FIELDS = {
  Abilities_iabi: 'iabi', // string list, comma-separated 4-char ability rawcodes, max 4
  ArmorType_iarm: 'iarm', // int 0-4, Flesh|Metal|Wood|Ethereal|Stone
  Classification_icla: 'icla', // int 0-6, Permanent|Charged|PowerUp|Artifact|Purchasable|Campaign|Miscellaneous
  TintingColor3Blue_iclb: 'iclb', // int, min 0, max 255
  TintingColor2Green_iclg: 'iclg', // int, min 0, max 255
  TintingColor1Red_iclr: 'iclr', // int, min 0, max 255
  CooldownGroup_icid: 'icid', // int 4-char rawcode
  DroppedWhenCarrierDies_idrp: 'idrp', // bool
  CanBeDropped_idro: 'idro', // bool
  ModelUsed_ifil: 'ifil', // string path (.mdl), import Model
  GoldCost_igol: 'igol', // int, min 0, max 100000
  HitPoints_ihtp: 'ihtp', // int, min 1, max 500000
  IgnoreCooldown_iicd: 'iicd', // bool
  Level_ilev: 'ilev', // int, min 0, max 8
  LumberCost_ilum: 'ilum', // int, min 0, max 100000
  ValidTargetForTransformation_imor: 'imor', // bool
  LevelUnclassified_ilvo: 'ilvo', // int, min 0, max 10
  Perishable_iper: 'iper', // bool
  IncludeAsRandomChoice_iprn: 'iprn', // bool
  UseAutomaticallyWhenAcquired_ipow: 'ipow', // bool
  Priority_ipri: 'ipri', // int, min 0, max 1000
  ScalingValue_isca: 'isca', // real, min 0.1, max 10
  SelSize_issc: 'issc', // real, min 0, max 10000
  CanBeSoldByMerchants_isel: 'isel', // bool
  CanBeSoldToMerchants_ipaw: 'ipaw', // bool
  StockMaximum_isto: 'isto', // int, min 0, max 100
  StockReplenishInterval_istr: 'istr', // int, min 0, max 3600
  StockStartDelay_isst: 'isst', // int, min 0, max 3600
  StockInitial_isit: 'isit', // int, min 1, max 100
  ActivelyUsed_iusa: 'iusa', // bool
  NumberofCharges_iuse: 'iuse', // int, min 0, max 1000
  StackMax_ista: 'ista', // int, min 0, max 1000
  RequiredAnimationNames_uani: 'uani', // string list, comma-separated tokens, max 20
  IconGameInterface_uico: 'uico', // string path (.blp), import Image
  InterfaceIcon_iico: 'iico', // string path (.blp), import Image
  RequiredAnimationNamesAttachments_uaap: 'uaap', // string list, comma-separated tokens, max 20
  RequiredAttachmentLinkNames_ualp: 'ualp', // string list, comma-separated tokens, max 20
  TooltipAwaken_uawt: 'uawt', // string, max TTDesc
  RequiredBoneNames_ubpr: 'ubpr', // string list, comma-separated tokens, max 20
  Construction_ubsl: 'ubsl', // string sound label key
  StructuresBuilt_ubui: 'ubui', // string list, comma-separated 4-char unit rawcodes
  ButtonPositionX_ubpx: 'ubpx', // int, min 0, max 3
  ButtonPositionY_ubpy: 'ubpy', // int, min 0, max 2
  CasterUpgradeArt_ucua: 'ucua', // string path (.blp), import Image
  CasterUpgradeNames_ucun: 'ucun', // string list, comma-separated tokens, max 32
  CasterUpgradeTips_ucut: 'ucut', // string list, comma-separated tokens, max TTDesc
  DependencyEquivalents_udep: 'udep', // string list, comma-separated 4-char unit rawcodes
  Description_ides: 'ides', // string, max TTDesc
  NameEditorSuffix_unsf: 'unsf', // string, max 50
  Hotkey_uhot: 'uhot', // char
  LoopingFadeInRate_ulfi: 'ulfi', // int, min 0, max 12700
  LoopingFadeOutRate_ulfo: 'ulfo', // int, min 0, max 12700
  ItemsMade_umki: 'umki', // string list, comma-separated 4-char item rawcodes, max 12
  Attack1ProjectileArc_uma1: 'uma1', // unreal, min 0, max 1
  Attack2ProjectileArc_uma2: 'uma2', // unreal, min 0, max 1
  Attack1ProjectileArt_ua1m: 'ua1m', // string path (.mdl), import Model
  Attack2ProjectileArt_ua2m: 'ua2m', // string path (.mdl), import Model
  Attack1ProjectileHomingEnabled_umh1: 'umh1', // bool
  Attack2ProjectileHomingEnabled_umh2: 'umh2', // bool
  Attack1ProjectileSpeed_ua1z: 'ua1z', // int, min 0, max 10000
  Attack2ProjectileSpeed_ua2z: 'ua2z', // int, min 0, max 10000
  Movement_umsl: 'umsl', // string sound label key
  Name_unam: 'unam', // string, max TTName
  ProperNames_upro: 'upro', // string list, comma-separated tokens, max 32
  Random_ursl: 'ursl', // string sound label key
  Requirescount_urqc: 'urqc', // int, max 9
  Requirements_ureq: 'ureq', // string list, comma-separated 4-char upgrade rawcodes
  Requires1_urq1: 'urq1', // string list, comma-separated 4-char upgrade rawcodes
  Requires2_urq2: 'urq2', // string list, comma-separated 4-char upgrade rawcodes
  Requires3_urq3: 'urq3', // string list, comma-separated 4-char upgrade rawcodes
  Requires4_urq4: 'urq4', // string list, comma-separated 4-char upgrade rawcodes
  Requires5_urq5: 'urq5', // string list, comma-separated 4-char upgrade rawcodes
  Requires6_urq6: 'urq6', // string list, comma-separated 4-char upgrade rawcodes
  Requires7_urq7: 'urq7', // string list, comma-separated 4-char upgrade rawcodes
  Requires8_urq8: 'urq8', // string list, comma-separated 4-char upgrade rawcodes
  RequirementsLevels_urqa: 'urqa', // string list, comma-separated integers
  ResearchesAvailable_ures: 'ures', // string list, comma-separated 4-char upgrade rawcodes
  RevivesDeadHeros_urev: 'urev', // bool
  TooltipRevive_utpr: 'utpr', // string, max TTDesc
  IconScoreScreen_ussi: 'ussi', // string path (.blp), import Image
  ItemsSold_usei: 'usei', // string list, comma-separated 4-char item rawcodes, max 12
  UnitsSold_useu: 'useu', // string list, comma-separated 4-char unit rawcodes, max 12
  Special_uspa: 'uspa', // string list, comma-separated model paths, import Model
  Target_utaa: 'utaa', // string list, comma-separated model paths, import Model
  TooltipBasic_utip: 'utip', // string, max TTDesc
  UnitsTrained_utra: 'utra', // string list, comma-separated 4-char unit rawcodes
  HeroRevivalLocations_urva: 'urva', // string list, comma-separated 4-char unit rawcodes
  TooltipExtended_utub: 'utub', // string, max TTUber
  UpgradesTo_uupt: 'uupt', // string list, comma-separated 4-char unit rawcodes, max 12
  Normal_uabi: 'uabi', // string list, comma-separated 4-char ability rawcodes
  DefaultActiveAbility_udaa: 'udaa', // int 4-char rawcode
  Hero_uhab: 'uhab', // string list, comma-separated 4-char ability rawcodes, max 5
  StartingAgility_uagi: 'uagi', // int, min 1, max 100
  AgilityperLevel_uagp: 'uagp', // unreal, min 0, max 20
  BuildTime_ubld: 'ubld', // int, min 1, max 298
  BountyAwardedNumberofDice_ubdi: 'ubdi', // int, min 0, max 100
  BountyAwardedBase_ubba: 'ubba', // int, min 0, max 10000
  BountyAwardedSidesperDie_ubsi: 'ubsi', // int, min 0, max 100
  Lumberbountydice_ulbd: 'ulbd', // int, min 0, max 100
  Lumberbountyplus_ulba: 'ulba', // int, min 0, max 10000
  Lumberbountysides_ulbs: 'ulbs', // int, min 0, max 100
  CollisionSize_ucol: 'ucol', // unreal, min 0, max 1024
  DefenseBase_udef: 'udef', // int, min 0, max 1000
  DefenseType_udty: 'udty', // int 0-7, normal|small|medium|large|fort|hero|divine|none
  DefenseUpgradeBonus_udup: 'udup', // int, min 0, max 1000
  FoodProduced_ufma: 'ufma', // int, min 0, max 300
  FoodCost_ufoo: 'ufoo', // int, min 0, max 300
  GoldCost_ugol: 'ugol', // int, min 0, max 100000
  RepairGoldCost_ugor: 'ugor', // int, min 0, max 100000
  HitPointsMaximumBase_uhpm: 'uhpm', // int, min 1, max 500000
  StartingIntelligence_uint: 'uint', // int, min 1, max 100
  IntelligenceperLevel_uinp: 'uinp', // unreal, min 0, max 20
  IsaBuilding_ubdg: 'ubdg', // bool
  Level_ulev: 'ulev', // int, min 1, max 100
  LumberCost_ulum: 'ulum', // int, min 0, max 100000
  RepairLumberCost_ulur: 'ulur', // int, min 0, max 100000
  ManaInitialAmount_umpi: 'umpi', // int, min 0, max 100000
  ManaMaximum_umpm: 'umpm', // int, min 0, max 100000
  SpeedMaximum_umas: 'umas', // int, min 0, max 522
  SpeedMinimum_umis: 'umis', // int, min 0, max 522
  NeutralBuildingValidAsRandomBuilding_unbr: 'unbr', // bool
  SightRadiusNight_usin: 'usin', // int, min 0, max 1800
  PlacementRequires_upap: 'upap', // string list, blighted|unbuildable|unflyable|unwalkable|unamph|unfloat
  PrimaryAttribute_upra: 'upra', // int 0-2, AGI|INT|STR
  HitPointsRegenerationRate_uhpr: 'uhpr', // unreal, min 0, max 1000
  ManaRegeneration_umpr: 'umpr', // unreal, min 0, max 1000
  HitPointsRegenerationType_uhrt: 'uhrt', // int 0-4, none|always|blight|day|night
  RepairTime_urtm: 'urtm', // int, min 1, max 10000
  GroupSeparationEnabled_urpo: 'urpo', // bool
  GroupSeparationGroupNumber_urpg: 'urpg', // int, min 0, max 1024
  GroupSeparationParameter_urpp: 'urpp', // int, min 0, max 4
  GroupSeparationPriority_urpr: 'urpr', // int, min 0, max 10000
  PlacementPreventedBy_upar: 'upar', // string list, blighted|unbuildable|unflyable|unwalkable|unamph|unfloat
  SightRadiusDay_usid: 'usid', // int, min 0, max 1800
  SpeedBase_umvs: 'umvs', // int, min 0, max 522
  StockMaximum_usma: 'usma', // int, min 0, max 32
  StockReplenishInterval_usrg: 'usrg', // int, min 0, max 3600
  StockStartDelay_usst: 'usst', // int, min 0, max 3600
  StockInitial_usit: 'usit', // int, min 1, max 32
  StartingStrength_ustr: 'ustr', // int, min 1, max 100
  StrengthperLevel_ustp: 'ustp', // unreal, min 0, max 20
  Tilesets_util: 'util', // string, *|A|B|C|D|L|N|O|U|Z tileset codes
  UnitClassification_utyp: 'utyp', // int 0-12, giant|undead|summoned|mechanical|peon|sapper|townhall|tree|ward|ancient|standon|neutral|tauren
  UpgradesUsed_upgr: 'upgr', // string list, comma-separated 4-char upgrade rawcodes
  AIPlacementRadius_uabr: 'uabr', // unreal, min 0, max 24
  AIPlacementType_uabt: 'uabt', // int 0-4, _|townhall|resource|factory|buffer
  CanBuildOn_ucbo: 'ucbo', // bool
  CanFlee_ufle: 'ufle', // bool
  Sleeps_usle: 'usle', // bool
  TransportedSize_ucar: 'ucar', // int, min 0, max 8
  DeathTimeseconds_udtm: 'udtm', // unreal, min 0.1, max 20
  DeathType_udea: 'udea', // int 0-3, 0|1|2|3
  UseExtendedLineofSight_ulos: 'ulos', // bool
  FormationRank_ufor: 'ufor', // int, min 0, max 9
  IsBuildOn_uibo: 'uibo', // bool
  HeightMinimum_umvf: 'umvf', // unreal, min -1000, max 1000
  Height_umvh: 'umvh', // unreal, min -1000, max 1000
  Type_umvt: 'umvt', // int 0-5, foot|horse|fly|hover|float|amph
  ProperNamesUsed_upru: 'upru', // int, min 0, max 1000
  OrientationInterpolation_uori: 'uori', // int, min 0, max 8
  PathingMap_upat: 'upat', // string path (.blp), import Image
  PointValue_upoi: 'upoi', // int, min 0, max 100000
  Priority_upri: 'upri', // int, min 0, max 20
  PropulsionWindowdegrees_uprw: 'uprw', // unreal, min 1, max 180
  Race_urac: 'urac', // int 0-10, human|orc|undead|nightelf|dem on|creeps|critters|other|commoner|naga|unknown
  PlacementRequiresWaterRadius_upaw: 'upaw', // unreal, min 0, max 2048
  Targetedas_utar: 'utar', // string list, air|alive|allies|dead|debris|enemies|ground|hero|invulnerable|item|mechanical|neutral|none|nonhero|nonsapper|notself|organic|player|sapper|self|structure|terrain|tree|vulnerable|wall|ward|ancient|nonancient|friend|bridge|decoration
  TurnRate_umvr: 'umvr', // unreal, min 0.1, max 3
  ArmorType_uarm: 'uarm', // int 0-4, Flesh|Metal|Wood|Ethereal|Stone
  AnimationBlendTimeseconds_uble: 'uble', // real, min 0, max 1
  TintingColor3Blue_uclb: 'uclb', // int, min 0, max 255
  ShadowTextureBuilding_ushb: 'ushb', // string path (.blp), import Image
  CategorizationCampaign_ucam: 'ucam', // bool
  AllowCustomTeamColor_utcc: 'utcc', // bool
  CanDropItemsOnDeath_udro: 'udro', // bool
  ElevationSamplePoints_uept: 'uept', // int, min 0, max 4
  ElevationSampleRadius_uerd: 'uerd', // real, min 0, max 2048
  ModelFile_umdl: 'umdl', // string path (.mdl), import Model
  ModelFileExtraVersions_uver: 'uver', // int 0-1, 0|1, min 0, max 3
  FogofWarSampleRadius_ufrd: 'ufrd', // real, min 0, max 2048
  TintingColor2Green_uclg: 'uclg', // int, min 0, max 255
  DisplayasNeutralHostile_uhos: 'uhos', // bool
  PlaceableInEditor_uine: 'uine', // bool
  MaximumPitchAngledegrees_umxp: 'umxp', // real, min 0, max 180
  MaximumRollAngledegrees_umxr: 'umxr', // real, min 0, max 180
  ScalingValue_usca: 'usca', // real, min 0.1, max 10
  NeutralBuildingShowsMinimapIcon_unbm: 'unbm', // bool
  HideHeroBar_uhhb: 'uhhb', // bool
  HideHeroMinimap_uhhm: 'uhhm', // bool
  HideHeroDeathMsg_uhhd: 'uhhd', // bool
  HideOnMinimap_uhom: 'uhom', // bool
  OccluderHeight_uocc: 'uocc', // unreal, min 0, max 2048
  TintingColor1Red_uclr: 'uclr', // int, min 0, max 255
  AnimationRunSpeed_urun: 'urun', // real, min 0, max 2000
  SelectionScale_ussc: 'ussc', // real, min 0.1, max 20
  ScaleProjectiles_uscb: 'uscb', // bool
  SelectionCircleOnWater_usew: 'usew', // bool
  SelectionCircleHeight_uslz: 'uslz', // real, min 0, max 2048
  ShadowImageHeight_ushh: 'ushh', // real, min 0, max 2048
  HasWaterShadow_ushr: 'ushr', // bool
  ShadowImageWidth_ushw: 'ushw', // real, min 0, max 2048
  ShadowImageCenterX_ushx: 'ushx', // real, min 0, max 2048
  ShadowImageCenterY_ushy: 'ushy', // real, min 0, max 2048
  CategorizationSpecial_uspe: 'uspe', // bool
  TeamColor_utco: 'utco', // int 0-13, -1|0|1|2|3|4|5|6|7|8|9|10|11|12
  HasTilesetSpecificData_utss: 'utss', // bool
  GroundTexture_uubs: 'uubs', // string 4-char ubersplat code
  ShadowImageUnit_ushu: 'ushu', // string enum, Shadow|ShadowFlyer, import Image
  UnitSoundSet_usnd: 'usnd', // string unit sound set key
  UseClickHelper_uuch: 'uuch', // bool
  AnimationWalkSpeed_uwal: 'uwal', // real, min 0, max 2000
  AcquisitionRange_uacq: 'uacq', // unreal, min 0, max 20000
  Attack1AttackType_ua1t: 'ua1t', // int 0-7, unknown|normal|pierce|siege|spells|chaos|magic|hero
  Attack2AttackType_ua2t: 'ua2t', // int 0-7, unknown|normal|pierce|siege|spells|chaos|magic|hero
  Attack1AnimationBackswingPoint_ubs1: 'ubs1', // unreal, min 0, max 10
  Attack2AnimationBackswingPoint_ubs2: 'ubs2', // unreal, min 0, max 10
  AnimationCastBackswing_ucbs: 'ucbs', // unreal, min 0, max 10
  AnimationCastPoint_ucpt: 'ucpt', // unreal, min 0, max 10
  Attack1CooldownTime_ua1c: 'ua1c', // unreal, min 0, max 3600
  Attack2CooldownTime_ua2c: 'ua2c', // unreal, min 0, max 3600
  Attack1DamageLossFactor_udl1: 'udl1', // unreal, min 0, max 10
  Attack2DamageLossFactor_udl2: 'udl2', // unreal, min 0, max 10
  Attack1DamageNumberofDice_ua1d: 'ua1d', // int, min 0, max 100
  Attack2DamageNumberofDice_ua2d: 'ua2d', // int, min 0, max 100
  Attack1DamageBase_ua1b: 'ua1b', // int, min 0, max 500000
  Attack2DamageBase_ua2b: 'ua2b', // int, min 0, max 500000
  Attack1AnimationDamagePoint_udp1: 'udp1', // unreal, min 0, max 10
  Attack2AnimationDamagePoint_udp2: 'udp2', // unreal, min 0, max 10
  Attack1DamageUpgradeAmount_udu1: 'udu1', // int, min 0, max 1000
  Attack2DamageUpgradeAmount_udu2: 'udu2', // int, min 0, max 1000
  Attack1AreaofEffectFullDamage_ua1f: 'ua1f', // int, min 0, max 1000
  Attack2AreaofEffectFullDamage_ua2f: 'ua2f', // int, min 0, max 1000
  Attack1AreaofEffectMediumDamage_ua1h: 'ua1h', // int, min 0, max 1000
  Attack2AreaofEffectMediumDamage_ua2h: 'ua2h', // int, min 0, max 1000
  Attack1DamageFactorMedium_uhd1: 'uhd1', // unreal, min 0, max 10
  Attack2DamageFactorMedium_uhd2: 'uhd2', // unreal, min 0, max 10
  ProjectileImpactZSwimming_uisz: 'uisz', // unreal, min -1000, max 1000
  ProjectileImpactZ_uimz: 'uimz', // unreal, min -1000, max 1000
  ProjectileLaunchZSwimming_ulsz: 'ulsz', // unreal, min -1000, max 1000
  ProjectileLaunchX_ulpx: 'ulpx', // unreal, min -1000, max 1000
  ProjectileLaunchY_ulpy: 'ulpy', // unreal, min -1000, max 1000
  ProjectileLaunchZ_ulpz: 'ulpz', // unreal, min -1000, max 1000
  MinimumAttackRange_uamn: 'uamn', // int, min 0, max 20000
  Attack1AreaofEffectSmallDamage_ua1q: 'ua1q', // int, min 0, max 1000
  Attack2AreaofEffectSmallDamage_ua2q: 'ua2q', // int, min 0, max 1000
  Attack1DamageFactorSmall_uqd1: 'uqd1', // unreal, min 0, max 10
  Attack2DamageFactorSmall_uqd2: 'uqd2', // unreal, min 0, max 10
  Attack1Range_ua1r: 'ua1r', // int, min 0, max 20000
  Attack2Range_ua2r: 'ua2r', // int, min 0, max 20000
  Attack1RangeMotionBuffer_urb1: 'urb1', // unreal, min 0, max 2000
  Attack2RangeMotionBuffer_urb2: 'urb2', // unreal, min 0, max 2000
  Attack1ShowUI_uwu1: 'uwu1', // bool
  Attack2ShowUI_uwu2: 'uwu2', // bool
  Attack1DamageSidesperDie_ua1s: 'ua1s', // int, min 0, max 100
  Attack2DamageSidesperDie_ua2s: 'ua2s', // int, min 0, max 100
  Attack1DamageSpillDistance_usd1: 'usd1', // unreal, min 0, max 10000
  Attack2DamageSpillDistance_usd2: 'usd2', // unreal, min 0, max 10000
  Attack1DamageSpillRadius_usr1: 'usr1', // unreal, min 0, max 10000
  Attack2DamageSpillRadius_usr2: 'usr2', // unreal, min 0, max 10000
  Attack1AreaofEffectTargets_ua1p: 'ua1p', // string list, air|alive|allies|dead|debris|enemies|ground|hero|invulnerable|item|mechanical|neutral|none|nonhero|nonsapper|notself|organic|player|sapper|self|structure|terrain|tree|vulnerable|wall|ward|ancient|nonancient|friend|bridge|decoration
  Attack2AreaofEffectTargets_ua2p: 'ua2p', // string list, air|alive|allies|dead|debris|enemies|ground|hero|invulnerable|item|mechanical|neutral|none|nonhero|nonsapper|notself|organic|player|sapper|self|structure|terrain|tree|vulnerable|wall|ward|ancient|nonancient|friend|bridge|decoration
  Attack1MaximumNumberofTargets_utc1: 'utc1', // int, min 0, max 100
  Attack2MaximumNumberofTargets_utc2: 'utc2', // int, min 0, max 100
  Attack1TargetsAllowed_ua1g: 'ua1g', // string list, air|alive|allies|dead|debris|enemies|ground|hero|invulnerable|item|mechanical|neutral|none|nonhero|nonsapper|notself|organic|player|sapper|self|structure|terrain|tree|vulnerable|wall|ward|ancient|nonancient|friend|bridge|decoration
  Attack2TargetsAllowed_ua2g: 'ua2g', // string list, air|alive|allies|dead|debris|enemies|ground|hero|invulnerable|item|mechanical|neutral|none|nonhero|nonsapper|notself|organic|player|sapper|self|structure|terrain|tree|vulnerable|wall|ward|ancient|nonancient|friend|bridge|decoration
  AttacksEnabled_uaen: 'uaen', // int 0-3, 0|1|2|3
  Attack1WeaponType_ua1w: 'ua1w', // int 0-7, normal|instant|artillery|aline|missile|msplash|mbounce|mline
  Attack2WeaponType_ua2w: 'ua2w', // int 0-7, normal|instant|artillery|aline|missile|msplash|mbounce|mline
  Attack1WeaponSound_ucs1: 'ucs1', // int 0-12, AxeMediumChop|MetalHeavyBash|MetalHeavyChop|MetalHeavySlice|MetalLightChop|MetalLightSlice|MetalMediumBash|MetalMediumChop|MetalMediumSlice|RockHeavyBash|WoodHeavyBash|WoodLightBash|WoodMediumBash
  Attack2WeaponSound_ucs2: 'ucs2', // int 0-12, AxeMediumChop|MetalHeavyBash|MetalHeavyChop|MetalHeavySlice|MetalLightChop|MetalLightSlice|MetalMediumBash|MetalMediumChop|MetalMediumSlice|RockHeavyBash|WoodHeavyBash|WoodLightBash|WoodMediumBash
  AbilSkinList_uabs: 'uabs', // string list, comma-separated 4-char ability rawcodes
  HeroAbilSkinList_uhas: 'uhas', // string list, comma-separated 4-char ability rawcodes
} as const;
