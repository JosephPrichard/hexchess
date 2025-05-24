package web.reusable;

import io.jooby.Context;
import io.jooby.Value;
import io.jooby.exception.BadRequestException;
import lombok.AllArgsConstructor;

import static utils.Globals.LOG;
import static web.WebConstants.*;

@AllArgsConstructor
public class PathService {

    public long getPathAsLong(Context ctx, String name) {
        try {
            String id = getPathAsString(ctx, name);
            return Long.parseUnsignedLong(id);
        } catch (NumberFormatException ex) {
            LOG.warn("Value with name={} is not a valid long", name);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }
    }

    public String getPathAsString(Context ctx, String name) {
        return getValueAsString(ctx.path(name), name);
    }

    public String getQueryAsString(Context ctx, String name) {
        return getValueAsString(ctx.query(name), name);
    }

    public String getValueAsString(Value value, String name) {
        if (value.isMissing()) {
            LOG.warn("Value is missing with name={}", name);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }
        return value.value();
    }

    public int getPageParam(Context ctx) {
        try {
            return ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            LOG.warn("Page value is not valid integer");
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }
    }
}
