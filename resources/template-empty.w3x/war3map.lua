gg_trg_Melee_Initialization = nil
gg_trg_LUA_map_init = nil
gg_trg_LUA_0s_elapse = nil
function InitGlobals()
end

function Trig_Melee_Initialization_Actions()
BlzShowTerrain(false)
CreateFogModifierRectBJ(true, Player(0), FOG_OF_WAR_VISIBLE, GetPlayableMapRect())
end

function InitTrig_Melee_Initialization()
gg_trg_Melee_Initialization = CreateTrigger()
TriggerAddAction(gg_trg_Melee_Initialization, Trig_Melee_Initialization_Actions)
end

function InitTrig_LUA_map_init()
end

function Trig_LUA_map_init_Actions()
    BlzChangeMinimapTerrainTex("minimap.blp")
    CinematicFadeBJ(bj_CINEFADETYPE_FADEOUT, 0, "ReplaceableTextures\\CameraMasks\\White_mask.blp", 0, 0, 0, 0)
    EnableUserControl(false)
end

function InitTrig_LUA_map_init()
    gg_trg_LUA_map_init = CreateTrigger()
    TriggerAddAction(gg_trg_LUA_map_init, Trig_LUA_map_init_Actions)
end


function InitTrig_LUA_0s_elapse()
end

function Trig_LUA_0s_elapse_Actions()
    -- WOW_CONVERTER_PRELOAD_LOCATIONS_BEGIN
    local locations = {}
    -- WOW_CONVERTER_PRELOAD_LOCATIONS_END
    local cameraX, cameraY = GetCameraTargetPositionX(), GetCameraTargetPositionY()
    local fields = {CAMERA_FIELD_TARGET_DISTANCE, CAMERA_FIELD_FARZ, CAMERA_FIELD_ANGLE_OF_ATTACK, CAMERA_FIELD_FIELD_OF_VIEW, CAMERA_FIELD_ROLL, CAMERA_FIELD_ROTATION, CAMERA_FIELD_ZOFFSET}
    local values = {}
    for i, field in ipairs(fields) do
        values[i] = GetCameraField(field)
    end
    local minX, minY = GetCameraBoundMinX(), GetCameraBoundMinY()
    local maxX, maxY = GetCameraBoundMaxX(), GetCameraBoundMaxY()
    SetCameraBoundsToRect(GetWorldBounds())
    local index = 1
    local timer = CreateTimer()
    TimerStart(timer, 0.10, true, function()
        local loc = locations[index]
        if loc then
            SetCameraPosition(loc[1], loc[2])
            index = index + 1
            return
        end
        DestroyTimer(timer)
        SetCameraBounds(minX, minY, maxX, maxY, minX, maxY, maxX, minY)
        for i, field in ipairs(fields) do
            local value = values[i]
            if field == CAMERA_FIELD_ANGLE_OF_ATTACK or field == CAMERA_FIELD_FIELD_OF_VIEW or field == CAMERA_FIELD_ROLL or field == CAMERA_FIELD_ROTATION then
                value = value * bj_RADTODEG
            end
            SetCameraField(field, value, 0)
        end
        SetCameraPosition(cameraX, cameraY)
        EnableUserControl(true)
        CinematicFadeBJ(bj_CINEFADETYPE_FADEIN, 0.50, "ReplaceableTextures\\CameraMasks\\White_mask.blp", 0, 0, 0, 0)
    end)
end

function InitTrig_LUA_0s_elapse()
    gg_trg_LUA_0s_elapse = CreateTrigger()
    TriggerRegisterTimerEventSingle(gg_trg_LUA_0s_elapse, 0.00)
    TriggerAddAction(gg_trg_LUA_0s_elapse, Trig_LUA_0s_elapse_Actions)
end


function InitCustomTriggers()
InitTrig_Melee_Initialization()
InitTrig_LUA_map_init()
InitTrig_LUA_0s_elapse()
end

function RunInitializationTriggers()
ConditionalTriggerExecute(gg_trg_Melee_Initialization)
ConditionalTriggerExecute(gg_trg_LUA_map_init)
end

function InitCustomPlayerSlots()
SetPlayerStartLocation(Player(0), 0)
SetPlayerColor(Player(0), ConvertPlayerColor(0))
SetPlayerRacePreference(Player(0), RACE_PREF_HUMAN)
SetPlayerRaceSkin(Player(0), RACE_PREF_USER_SELECTABLE)
SetPlayerRaceSelectable(Player(0), true)
SetPlayerController(Player(0), MAP_CONTROL_USER)
end

function InitCustomTeams()
SetPlayerTeam(Player(0), 0)
end

function main()
SetCameraBounds(-3328.0 + GetCameraMargin(CAMERA_MARGIN_LEFT), -3584.0 + GetCameraMargin(CAMERA_MARGIN_BOTTOM), 3328.0 - GetCameraMargin(CAMERA_MARGIN_RIGHT), 3072.0 - GetCameraMargin(CAMERA_MARGIN_TOP), -3328.0 + GetCameraMargin(CAMERA_MARGIN_LEFT), 3072.0 - GetCameraMargin(CAMERA_MARGIN_TOP), 3328.0 - GetCameraMargin(CAMERA_MARGIN_RIGHT), -3584.0 + GetCameraMargin(CAMERA_MARGIN_BOTTOM))
SetDayNightModels("Environment\\DNC\\DNCLordaeron\\DNCLordaeronTerrain\\DNCLordaeronTerrain.mdl", "Environment\\DNC\\DNCLordaeron\\DNCLordaeronUnit\\DNCLordaeronUnit.mdl")
SetHDWaterParamsEx(0, 0, 0, false, 20, 0, 100, 10, 0, 50, 100, 100)
NewSoundEnvironment("Default")
SetAmbientDaySound("LordaeronSummerDay")
SetAmbientNightSound("LordaeronSummerNight")
SetMapMusic("Music", true, 0)
InitBlizzard()
InitGlobals()
InitCustomTriggers()
RunInitializationTriggers()
end

function config()
SetMapName("TRIGSTR_003")
SetMapDescription("TRIGSTR_005")
SetPlayers(1)
SetTeams(1)
SetGamePlacement(MAP_PLACEMENT_USE_MAP_SETTINGS)
DefineStartLocation(0, 320.0, 64.0)
InitCustomPlayerSlots()
SetPlayerSlotAvailable(Player(0), MAP_CONTROL_USER)
InitGenericPlayerSlots()
end

