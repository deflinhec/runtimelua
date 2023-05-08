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