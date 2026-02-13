import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'getting-started/overview',
        'getting-started/prerequisites',
        'getting-started/installation',
      ],
    },
    {
      type: 'category',
      label: 'Configuration',
      items: [
        'configuration/helm-values',
        'configuration/admin-config',
        'configuration/networking',
        'configuration/backups',
        'configuration/auth',
      ],
    },
    {
      type: 'category',
      label: 'Usage',
      items: [
        'usage/creating-servers',
        'usage/managing-servers',
        'usage/backups-restore',
        'usage/admin-tasks',
      ],
    },
    {
      type: 'category',
      label: 'Contributing',
      items: [
        'contributing/game-definitions',
        'contributing/development',
        'contributing/architecture',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/api-endpoints',
        'reference/crd-reference',
        'reference/metrics',
      ],
    },
  ],
};

export default sidebars;
