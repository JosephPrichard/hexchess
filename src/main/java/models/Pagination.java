package models;

import lombok.AllArgsConstructor;
import lombok.Data;

import java.util.ArrayList;
import java.util.List;

@Data
@AllArgsConstructor
public class Pagination {
    public List<Page> pages;
    public String leftPage;
    public String rightPage;

    @Data
    @AllArgsConstructor
    public static class Page {
        String location;
        int value;

        public static Page of(String baseUrl, int page) {
            return new Page(createURL(baseUrl, page), page);
        }
    }

    public static String createURL(String baseUrl, int page) {
        return baseUrl + "page=" + page;
    }

    public static Pagination withTotal(String baseUrl, int targetPage, int totalPages) {
        List<Page> pages = new ArrayList<>();
        for (int page = Math.max(targetPage - 2, 1); page <= Math.min(targetPage + 2, totalPages); page++) {
            pages.add(Page.of(baseUrl, page));
        }
        return new Pagination(pages,
            targetPage > 1 ? createURL(baseUrl, targetPage - 1) : null,
            targetPage < totalPages ? createURL(baseUrl, targetPage + 1) : null);
    }

    public static Pagination ofUnlimited(String baseUrl, int page) {
        return new Pagination(
            List.of(Page.of(baseUrl, page)),
            page > 1 ? createURL(baseUrl, page - 1) : null,
            createURL(baseUrl, page + 1));
    }
}