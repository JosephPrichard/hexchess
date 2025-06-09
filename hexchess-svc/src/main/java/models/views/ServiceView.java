package models.views;

import lombok.AllArgsConstructor;
import lombok.Data;

@Data
@AllArgsConstructor
public class ServiceView {
    private int status;
    private String message = "";

    public static final ServiceView SUCCESS = new ServiceView(200, "SUCCESS");
}