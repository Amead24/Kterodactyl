import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

const features = [
  {
    title: 'Self-Service Provisioning',
    description:
      'Users browse a catalog of game definitions, configure server parameters via dynamic forms, and launch servers with a single click.',
  },
  {
    title: 'Community Game Definitions',
    description:
      'Game definitions are YAML manifests with JSON Schema for parameters. Adding a new game is as simple as contributing a manifest file.',
  },
  {
    title: 'Dynamic UI from Manifests',
    description:
      'The React frontend generates configuration forms directly from JSON Schema embedded in game manifests. No UI code changes needed for new games.',
  },
  {
    title: 'Helm-Based Installation',
    description:
      'Deploy everything with a single Helm chart. The operator, API server, and embedded frontend are packaged together for easy installation.',
  },
  {
    title: 'Built-In Backups and Mods',
    description:
      'S3-compatible backup system with scheduled and on-demand backups. Per-server mod storage with upload, list, and delete support.',
  },
];

function HomepageHeader(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className="hero hero--primary" style={{padding: '4rem 0'}}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <p style={{fontSize: '1.2rem', maxWidth: '600px', margin: '0 auto 2rem'}}>
          A Kubernetes-native game server management panel. An open-source
          alternative to Pterodactyl that replaces Wings, Docker, and Postgres
          with CRDs, an operator, and Kubernetes primitives.
        </p>
        <div>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started/overview">
            Get Started
          </Link>
        </div>
      </div>
    </header>
  );
}

function Feature({title, description}: {title: string; description: string}): ReactNode {
  return (
    <div className="col col--4" style={{marginBottom: '2rem'}}>
      <Heading as="h3">{title}</Heading>
      <p>{description}</p>
    </div>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      title="Home"
      description="Kubernetes-native game server management panel">
      <HomepageHeader />
      <main>
        <section style={{padding: '2rem 0'}}>
          <div className="container">
            <div className="row">
              {features.map((f, idx) => (
                <Feature key={idx} title={f.title} description={f.description} />
              ))}
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}
