local pretty_print, extract_constant_table, table_to_json, err = dofile("./build/lua_scripts/base_extractor.lua")

local target_path = "./assets/data/scripts/gun/gun_actions.lua"
local constants_path = "./assets/data/scripts/gun/gun_enums.lua"
local table_name = "actions"

print("--- Start ---")
local extracted_actions = extract_constant_table(target_path, constants_path, table_name)
print("--- End ---")

if extracted_actions then
    -- pretty_print(extracted_actions)
    local action_info = {}
    for _, value in ipairs(extracted_actions) do
        table.insert(action_info, {id = value.id, name = value.name, description = value.description, sprite = value.sprite, type = value.type})
    end

    local target_file = io.open("./assets/spells_raw.json", "w")
    if target_file then
        target_file:write(table_to_json(action_info) or "[]")
        target_file:close()
        print("\n-> Success")
        return
    end
end
print("\n-> Fail")