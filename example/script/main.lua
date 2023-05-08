-- Copyright 2023 Deflinhec
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
-- http:--www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.

local runtime = require 'runtime'
local event = require 'event'
local logger = require 'logger'

local ticks = 0
event.loop(1, function(timer)
    ticks = ticks + 1
    if ticks >= 10 then
        logger.info("tick", {
            ticks = ticks,
            status = "stop",
        })
        runtime.exit()
        return false
    end
    logger.info("tick", {
        ticks = ticks,
        status = "active",
    })
    return true
end)