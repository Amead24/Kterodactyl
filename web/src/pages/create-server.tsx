import { useState } from 'react';
import { useNavigate, useSearchParams, Link } from 'react-router';
import { ArrowLeft, Gamepad2 } from 'lucide-react';
import { toast } from 'sonner';
import type { RJSFSchema } from '@rjsf/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { GameConfigForm } from '@/components/forms/game-config-form';
import { useGames, useGame } from '@/hooks/use-games';
import { useCreateServer } from '@/hooks/use-servers';
import type { GameResponse } from '@/types/api';

/** DNS label validation: lowercase alphanumeric and hyphens, must start/end with alphanumeric. */
const DNS_LABEL_REGEX = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

export default function CreateServerPage() {
  const [searchParams] = useSearchParams();
  const selectedGame = searchParams.get('game');

  // If a game is selected via query param, show the config form
  if (selectedGame) {
    return <ConfigureStep gameType={selectedGame} />;
  }

  return <GameSelectionStep />;
}

/** Step 1: Select a game from the available games list. */
function GameSelectionStep() {
  const { data, isLoading, error } = useGames();

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/servers">
            <ArrowLeft className="mr-1 size-4" />
            Back
          </Link>
        </Button>
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Create Server</h1>
          <p className="text-muted-foreground">
            Select a game to create a new server
          </p>
        </div>
      </div>

      {isLoading && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-32 rounded-xl" />
          ))}
        </div>
      )}

      {error && (
        <p className="text-sm text-destructive">
          Failed to load games: {error.message}
        </p>
      )}

      {data && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {data.data.map((game) => (
            <GameSelectCard key={game.name} game={game} />
          ))}
        </div>
      )}

      {data && data.data.length === 0 && (
        <p className="py-10 text-center text-muted-foreground">
          No games available. Contact your administrator.
        </p>
      )}
    </div>
  );
}

/** Selectable game card for the game selection step. */
function GameSelectCard({ game }: { game: GameResponse }) {
  return (
    <Link to={`/servers/create?game=${game.name}`}>
      <Card className="cursor-pointer transition-colors hover:border-primary">
        <CardHeader>
          <CardTitle className="text-lg">{game.displayName}</CardTitle>
          <CardDescription>{game.name}</CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          {game.ports.length > 0 && (
            <p>
              Ports:{' '}
              {game.ports
                .map((p) => `${p.containerPort}/${p.protocol}`)
                .join(', ')}
            </p>
          )}
        </CardContent>
      </Card>
    </Link>
  );
}

/** Step 2: Configure and create the server for the selected game. */
function ConfigureStep({ gameType }: { gameType: string }) {
  const navigate = useNavigate();
  const { data: game, isLoading, error } = useGame(gameType);
  const createMutation = useCreateServer();
  const [serverName, setServerName] = useState('');
  const [nameError, setNameError] = useState<string | null>(null);

  function validateName(value: string): boolean {
    if (!value) {
      setNameError('Server name is required');
      return false;
    }
    if (value.length > 63) {
      setNameError('Server name must be 63 characters or fewer');
      return false;
    }
    if (!DNS_LABEL_REGEX.test(value)) {
      setNameError(
        'Must be lowercase, alphanumeric, and hyphens only. Must start and end with a letter or number.',
      );
      return false;
    }
    setNameError(null);
    return true;
  }

  function handleSubmit(parameters: Record<string, string>) {
    if (!validateName(serverName)) return;

    createMutation.mutate(
      {
        name: serverName,
        gameType,
        parameters: Object.keys(parameters).length > 0 ? parameters : undefined,
      },
      {
        onSuccess: () => {
          toast.success(`Server "${serverName}" created successfully`);
          navigate(`/servers/${serverName}`);
        },
      },
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    );
  }

  if (error || !game) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-destructive">
          {error ? `Failed to load game: ${error.message}` : 'Game not found'}
        </p>
        <Button variant="outline" asChild>
          <Link to="/servers/create">
            <ArrowLeft className="mr-1 size-4" />
            Back to game selection
          </Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/servers/create">
            <ArrowLeft className="mr-1 size-4" />
            Back
          </Link>
        </Button>
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Create Server</h1>
          <p className="text-muted-foreground">
            Configure your {game.displayName} server
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Gamepad2 className="size-5" />
            <CardTitle>{game.displayName}</CardTitle>
          </div>
          <CardDescription>
            Image: {game.image}
            {game.ports.length > 0 &&
              ` | Ports: ${game.ports.map((p) => `${p.containerPort}/${p.protocol}`).join(', ')}`}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Server name */}
          <div className="space-y-2">
            <Label htmlFor="server-name">Server Name</Label>
            <Input
              id="server-name"
              placeholder="my-game-server"
              value={serverName}
              onChange={(e) => {
                setServerName(e.target.value.toLowerCase());
                if (nameError) validateName(e.target.value.toLowerCase());
              }}
              onBlur={() => {
                if (serverName) validateName(serverName);
              }}
            />
            {nameError && (
              <p className="text-sm text-destructive">{nameError}</p>
            )}
            <p className="text-xs text-muted-foreground">
              Lowercase letters, numbers, and hyphens. Must start and end with a
              letter or number.
            </p>
          </div>

          {/* Dynamic config form from parameterSchema */}
          <div className="space-y-2">
            <Label>Configuration</Label>
            <GameConfigForm
              parameterSchema={(game.parameterSchema as RJSFSchema) ?? {}}
              defaultParameters={game.parameters ?? {}}
              onSubmit={handleSubmit}
              isLoading={createMutation.isPending}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
