export interface ReplaysQuery {
    userId?: string;
    whiteId?: string;
    blackId?: string;
    winnerId?: string;
    loserId?: string;

    whitename?: string;
    blackname?: string;
    winnername?: string;
    losername?: string;

    fromDate?: string;
    toDate?: string;
    mode?: string;
    result?: string;
    cause?: string;
    afterId?: string;
    afterTurnCount?: string;
    afterRating?: string;

    sort?: string;
}