import { Link } from 'react-router';
import { Gamepad2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type { GameResponse } from '@/types/api';

interface GameCardProps {
  game: GameResponse;
}

/** Card displaying a game with its type, ports, parameter count, and create action. */
export function GameCard({ game }: GameCardProps) {
  const paramCount = game.parameters ? Object.keys(game.parameters).length : 0;

  return (
    <Card className="flex flex-col">
      <CardHeader>
        <div className="flex items-start justify-between gap-2">
          <CardTitle className="text-lg">{game.displayName}</CardTitle>
          <Badge variant="secondary">{game.name}</Badge>
        </div>
      </CardHeader>
      <CardContent className="flex-1 space-y-2 text-sm text-muted-foreground">
        {game.ports.length > 0 && (
          <p>
            Ports:{' '}
            {game.ports
              .map((p) => `${p.containerPort}/${p.protocol}`)
              .join(', ')}
          </p>
        )}
        {paramCount > 0 && (
          <p>{paramCount} configurable parameter{paramCount !== 1 ? 's' : ''}</p>
        )}
      </CardContent>
      <CardFooter>
        <Button asChild className="w-full">
          <Link to={`/servers/create?game=${game.name}`}>
            <Gamepad2 className="mr-2 size-4" />
            Create Server
          </Link>
        </Button>
      </CardFooter>
    </Card>
  );
}
