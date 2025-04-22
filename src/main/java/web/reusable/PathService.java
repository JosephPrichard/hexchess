package web.reusable;

import com.github.jknack.handlebars.Template;
import io.jooby.Context;
import io.jooby.StatusCode;
import io.jooby.Value;
import io.jooby.exception.BadRequestException;
import lombok.AllArgsConstructor;
import web.Templates;
import web.controllers.PageController;

import java.io.IOException;

import static web.Templates.*;

@AllArgsConstructor
public class PathService {
    private Templates templates;

    public long getIdPath(Context ctx) throws IOException {
        Template template = templates.getErrorTemplate();

        try {
            Value idPath = ctx.path("id");
            if (idPath.isMissing()) {
                String resp = template.apply(new ErrorPage(StatusCode.BAD_REQUEST_CODE, "Invalid param id: must contain id within path parameter."));
                throw new BadRequestException(resp);
            }
            return Long.parseUnsignedLong(idPath.value());
        } catch (NumberFormatException ex) {
            String resp = template.apply(new ErrorPage(StatusCode.BAD_REQUEST_CODE, "Invalid param id: must contain id within path parameter."));
            throw new BadRequestException(resp);
        }
    }

    public int getPageParam(Context ctx) throws IOException {
        Template template = templates.getErrorTemplate();

        try {
            return ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            String resp = template.apply(new ErrorPage(StatusCode.BAD_REQUEST_CODE, "Invalid param page: must be a positive integer."));
            throw new BadRequestException(resp);
        }
    }
}
