-- 一个通用的 pretty-print 函数，用于优雅地打印 table 的内容
---@param data any 要打印的数据
---@param indent number|nil 当前缩进级别
local function pretty_print(data, indent)
    indent = indent or 0
    local indent_str = string.rep("  ", indent)
    if type(data) ~= "table" then
        print(indent_str .. tostring(data))
        return
    end

    print(indent_str .. "{")
    for k, v in pairs(data) do
        local key_str = type(k) == "string" and ("['%s']"):format(k) or ("[%s]"):format(tostring(k))
        io.write(indent_str .. "  " .. key_str .. " = ")
        if type(v) == "table" then
            print()
            pretty_print(v, indent + 2)
        else
            print(tostring(v) .. ",")
        end
    end
    print(indent_str .. "},")
end

-- 从指定文件中安全地提取一个 table 的常量部分
---@param target_file_path string 目标文件路径
---@param constants_file_path string 提供常量的文件路径
---@param target_table_name string 要提取的 table 的名称
---@return table|nil extracted_table 提取并清理后的 table，如果失败则返回 nil
local function extract_constant_table(target_file_path, constants_file_path, target_table_name)
    local captured_table = nil

    -- 1. 创建黑洞代理和空函数，用于处理未知变量和函数调用
    local dummy_func = function() end
    local proxy_table = {}
    local proxy_metatable = {
        __index = function() return dummy_func end, -- 读取任何字段都返回一个空函数
        __newindex = function() end, -- 写入任何字段都直接忽略
        __call = function() end, -- 自身被调用时也什么都不做
    }
    setmetatable(proxy_table, proxy_metatable)

    -- 2. 创建沙箱环境
    local sandbox_env = {}

    -- 3. 预填充已知的常量
    -- 使用 loadfile 在一个空环境中执行常量文件，避免污染当前环境
    local const_chunk, err = loadfile(constants_file_path)
    if not const_chunk then
        print("Error loading constants file: " .. err)
        return nil
    end
    setfenv(const_chunk, sandbox_env)
    const_chunk() -- 执行后，常量就被加载到 sandbox_env 中了

    -- 4. 设置沙箱环境的元表，以捕获所有未知变量的读写
    setmetatable(sandbox_env, {
        __index = function(_, key)
            -- 当读取一个 sandbox_env 中不存在的 key 时
            -- 返回代理表，这样 c.fire_rate_wait 等操作不会失败
            return proxy_table
        end,
        __newindex = function(t, key, value)
            -- 当对 sandbox_env 进行写操作时 (例如 actions = {...})
            if key == target_table_name then
                -- 捕获我们想要的目标表
                captured_table = value
            else
                -- 其他的全局变量赋值，直接在沙箱中设置即可
                rawset(t, key, value)
            end
        end
    })

    -- 5. 加载目标文件并设置其环境
    local chunk, err = loadfile(target_file_path)
    if not chunk then
        print("Error loading target file: " .. err)
        return nil
    end
    -- 将代码块的环境设置为我们的沙箱
    setfenv(chunk, sandbox_env)

    -- 6. 在保护模式下执行代码块
    local success, exec_err = pcall(chunk)
    if not success then
        print("Error executing target file chunk: " .. tostring(exec_err))
        -- 即使执行出错，我们可能也已经捕获到了表，所以继续执行
    end

    if not captured_table then
        print("Could not capture table '" .. target_table_name .. "'.")
        return nil
    end

    -- 7. 后处理：递归地移除所有函数
    local function strip_functions(tbl)
        if type(tbl) ~= "table" then return end
        
        local keys_to_remove = {}
        for k, v in pairs(tbl) do
            if type(v) == "function" then
                table.insert(keys_to_remove, k)
            elseif type(v) == "table" then
                strip_functions(v) -- 递归处理子表
            end
        end

        for _, k in ipairs(keys_to_remove) do
            tbl[k] = nil
        end

        return tbl
    end

    return strip_functions(captured_table)
end

-- 将一个纯数据的 Lua table 转换为 JSON 格式的字符串。
-- - 如果 table 的键是连续的从 1 开始的整数，则视其为数组 (JSON Array)。
-- - 否则，视其为对象 (JSON Object)。
-- - 函数会处理字符串转义、数字、布尔值和 nil。
-- - 注意：此函数不处理 function, userdata, thread 等复杂类型，也不处理循环引用的 table。
---@param data table 要转换的纯数据 Lua 表。
---@return string|nil json_string 返回 JSON 格式的字符串，如果出错则返回 nil。
local function table_to_json(data)
    -- 内窥函数，用于处理不同类型的值
    local function serialize(value)
        local t = type(value)
        if t == "string" then
            -- 对字符串进行转义
            return '"' .. value:gsub('[\\"]', {['\\'] = '\\\\', ['"'] = '\\"'}):gsub('\n', '\\n'):gsub('\r', '\\r'):gsub('\t', '\\t') .. '"'
        elseif t == "number" then
            -- 检查是否是无穷大或 NaN，JSON 不支持这些
            if value ~= value or value == math.huge or value == -math.huge then
                return "null"
            end
            return tostring(value)
        elseif t == "boolean" then
            return tostring(value)
        elseif t == "nil" then
            return "null"
        elseif t == "table" then
            -- 递归处理 table
            return table_to_json(value)
        else
            -- 不支持的类型返回 null
            return "null"
        end
    end

    -- 检查是否为数组
    local is_array = true
    local key_count = 0
    local max_int_key = 0
    for k, _ in pairs(data) do
        -- 只要发现一个非整数键，就肯定不是数组
        if type(k) ~= "number" or k < 1 or k % 1 ~= 0 then
            is_array = false
            break
        end
        if k > max_int_key then
            max_int_key = k
        end
        key_count = key_count + 1
    end
    -- 如果在循环中没有提前退出，则根据键的连续性做最终判断
    -- 如果最大整数键和键的总数相等，说明是 1..N 的连续序列
    is_array = is_array and key_count == max_int_key

    local parts = {}
    if is_array then
        -- 序列化为 JSON Array
        for i = 1, key_count do
            table.insert(parts, serialize(data[i]))
        end
        return '[' .. table.concat(parts, ',') .. ']'
    else
        -- 序列化为 JSON Object
        for key, value in pairs(data) do
            if type(key) ~= "string" and type(key) ~= "number" then
                -- JSON 对象的键必须是字符串
                -- 我们跳过非字符串/数字的键
            else
                local json_key = '"' .. tostring(key) .. '"'
                table.insert(parts, json_key .. ':' .. serialize(value))
            end
        end
        return '{' .. table.concat(parts, ',') .. '}'
    end
end

local target_path = "./assets/data/scripts/gun/gun_actions.lua"
local constants_path = "./assets/data/scripts/gun/gun_enums.lua"

print("--- Start ---")
local extracted_actions = extract_constant_table(target_path, constants_path, "actions")
print("--- End ---\n")

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
        print("Success")
        return
    end
end
print("Fail")