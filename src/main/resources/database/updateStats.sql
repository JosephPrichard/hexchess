-- Utility function to calculate the probability of a win used in the elo formula within the update stats transaction
CREATE OR REPLACE FUNCTION probabilityWins(IN elo1 NUMERIC, IN elo2 NUMERIC)
    RETURNS NUMERIC
    LANGUAGE plpgsql
AS $$
BEGIN
    RETURN 1.0 / (1.0 + POWER(10, (elo1 - elo2) / 400.0));
END $$;

-- Transaction to calculate the new stats of a winner and loser of a game, returning the new stats of each player
CREATE OR REPLACE PROCEDURE updateStats(
    IN winId VARCHAR,
    IN loseId VARCHAR,
    OUT winEloNext NUMERIC,
    OUT loseEloNext NUMERIC
)
    LANGUAGE plpgsql
AS $$
DECLARE
    winElo NUMERIC;
    loseElo NUMERIC;
BEGIN
    SELECT elo INTO winElo FROM users WHERE id = winId;
    SELECT elo INTO loseElo FROM users WHERE id = loseId;

    winEloNext = winElo + (30 * (1 - probabilityWins(loseElo, winElo)));
    loseEloNext = loseElo + ((30 * probabilityWins(winElo, loseElo)) * -1);

    UPDATE users
    SET elo = winEloNext, wins = wins + 1, highestElo = GREATEST(highestElo, winEloNext)
    WHERE id = winId;

    UPDATE users
    SET elo = loseEloNext, losses = losses + 1
    WHERE id = loseId;
END $$;
END;