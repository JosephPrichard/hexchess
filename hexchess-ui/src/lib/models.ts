export interface Session {
    sessionId: string;
    userId: number;
    username: string;
    country: string;
}

export type ColorSelect = 'RANDOM' | 'WHITE' | 'BLACK';

export type TimeControl = 'UNLIMITED' | 'REAL_TIME' | 'CORRESPONDENCE';

export const Piece = {
    empty: 0,
    whitePawn: 1,
    blackPawn: 2,
    whiteKnight: 3,
    blackKnight: 4,
    whiteBishop: 5,
    blackBishop: 6,
    whiteRook: 7,
    blackRook: 8,
    whiteQueen: 9,
    blackQueen: 10,
    whiteKing: 11,
    blackKing: 12
} as const;

export type PieceType = (typeof Piece)[keyof typeof Piece];

export const pieceNames: Record<PieceType, string> = {
    [Piece.empty]: 'empty',
    [Piece.whitePawn]: 'white-pawn',
    [Piece.blackPawn]: 'black-pawn',
    [Piece.whiteKnight]: 'white-knight',
    [Piece.blackKnight]: 'black-knight',
    [Piece.whiteBishop]: 'white-bishop',
    [Piece.blackBishop]: 'black-bishop',
    [Piece.whiteRook]: 'white-rook',
    [Piece.blackRook]: 'black-rook',
    [Piece.whiteQueen]: 'white-queen',
    [Piece.blackQueen]: 'black-queen',
    [Piece.whiteKing]: 'white-king',
    [Piece.blackKing]: 'black-king'
};

export type Turn = 'white' | 'black';

export interface ChessBoard {
    turn: Turn;
    pieces: PieceType[][];
}

export interface Hexagon {
    file: number;
    rank: number;
}

export interface Move {
    piece: PieceType;
    from: Hexagon;
    to: Hexagon;
}

export interface UserView {
    id: number;
    username: string;
    country: string;
    elo: number;
    highestElo: number;
    wins: number;
    losses: number;
    rank: number;
    bio: string;
    joinedOn: string;
    total: number;
    winRate: number;
    winRateColor: string;
}

export interface ChallengeView {
    challengerId: number;
    challengerName: string;
    challengerCountry: string;
    challengerElo: number;
    challengeeId: number;
    challengeeName: string;
    challengeeCountry: string;
    challengeeElo: number;
    timeControl: string;
    madeAgo: string;
    expiresIn: string;
}

export interface ReplayView {
    id: number;
    whiteId: number;
    blackId: number;
    whiteName: string;
    blackName: string;
    whiteCountry: string;
    blackCountry: string;
    winElo: number;
    loseElo: number;
    whiteElo: string;
    blackElo: string;
    moveList?: Move[];
    playedOn: string;
    result: string;
    cause: string;
    whiteEloDiff: string;
    blackEloDiff: string;
    whiteEloColor: string;
    blackEloColor: string;
}

export interface UserWithReplaysView {
    user?: UserView;
    replayList: ReplayView[];
}

export interface WsMessage {
    type: string;
}