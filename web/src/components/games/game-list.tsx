import { Skeleton } from '@/components/ui/skeleton';
import { GameCard } from '@/components/games/game-card';
import { useGames } from '@/hooks/use-games';

/** Responsive grid of game cards, fetched via TanStack Query. */
export function GameList() {
  const { data, isLoading, error } = useGames();

  if (isLoading) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-52 rounded-xl" />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <p className="text-sm text-destructive">
        Failed to load games: {error.message}
      </p>
    );
  }

  const games = data?.data ?? [];

  if (games.length === 0) {
    return (
      <p className="py-10 text-center text-muted-foreground">
        No games available
      </p>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {games.map((game) => (
        <GameCard key={game.name} game={game} />
      ))}
    </div>
  );
}
