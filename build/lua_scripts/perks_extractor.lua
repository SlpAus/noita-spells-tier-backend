local pretty_print, extract_constant_table, table_to_json, err = dofile("./build/lua_scripts/base_extractor.lua")

local target_path = "./assets/data/scripts/perks/perk_list.lua"
local constants_path = nil
local table_name = "perk_list"

print("--- Start ---")
local extracted_perks = extract_constant_table(target_path, constants_path, table_name)
print("--- End ---")

if extracted_perks then
    -- pretty_print(extracted_actions)
    local perk_info = {}
    for _, value in ipairs(extracted_perks) do
        if not value.not_in_default_perk_pool then
            table.insert(perk_info, {id = value.id, name = value.ui_name, description = value.ui_description, sprite = value.perk_icon, type = 0})
        end
    end

    local target_file = io.open("./assets/perks_raw.json", "w")
    if target_file then
        target_file:write(table_to_json(perk_info) or "[]")
        target_file:close()
        print("\n-> Success")
        return
    end
end
print("\n-> Fail")