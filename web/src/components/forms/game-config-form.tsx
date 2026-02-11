import { withTheme } from '@rjsf/core';
import type { IChangeEvent } from '@rjsf/core';
import { Theme as ShadcnTheme } from '@rjsf/shadcn';
import validator from '@rjsf/validator-ajv8';
import type { RJSFSchema } from '@rjsf/utils';
import { Button } from '@/components/ui/button';
import { Loader2 } from 'lucide-react';

const Form = withTheme(ShadcnTheme);

interface GameConfigFormProps {
  parameterSchema: RJSFSchema;
  defaultParameters: Record<string, string>;
  onSubmit: (parameters: Record<string, string>) => void;
  isLoading?: boolean;
}

/**
 * Dynamic form generated from a game's parameterSchema using react-jsonschema-form.
 *
 * Uses the default draft-07 validator (schemas only use draft-07 features:
 * enum, const, pattern, maxLength, default, type, required).
 */
export function GameConfigForm({
  parameterSchema,
  defaultParameters,
  onSubmit,
  isLoading,
}: GameConfigFormProps) {
  const hasSchema =
    parameterSchema &&
    typeof parameterSchema === 'object' &&
    Object.keys(parameterSchema).length > 0 &&
    parameterSchema.properties &&
    Object.keys(parameterSchema.properties).length > 0;

  if (!hasSchema) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-muted-foreground">
          No configurable parameters for this game.
        </p>
        <Button
          type="button"
          onClick={() => onSubmit({})}
          disabled={isLoading}
        >
          {isLoading && <Loader2 className="mr-2 size-4 animate-spin" />}
          Create Server
        </Button>
      </div>
    );
  }

  function handleSubmit({ formData }: IChangeEvent) {
    onSubmit(formData ?? {});
  }

  return (
    <Form
      schema={parameterSchema}
      formData={defaultParameters}
      validator={validator}
      onSubmit={handleSubmit}
      disabled={isLoading}
    >
      <Button type="submit" disabled={isLoading}>
        {isLoading && <Loader2 className="mr-2 size-4 animate-spin" />}
        Create Server
      </Button>
    </Form>
  );
}
