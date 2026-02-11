import { GameList } from '@/components/games/game-list';

/** Game browser page -- shows available games as cards in a responsive grid. */
export default function GamesPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Available Games</h1>
        <p className="text-muted-foreground">
          Choose a game to create a new server
        </p>
      </div>
      <GameList />
    </div>
  );
}
