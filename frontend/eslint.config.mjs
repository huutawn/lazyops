import nextCoreWebVitals from 'eslint-config-next/core-web-vitals';

const eslintConfig = [
  {
    ignores: ['.next/**', 'coverage/**', 'node_modules/**', 'public/mockServiceWorker.js'],
  },
  ...nextCoreWebVitals,
];

export default eslintConfig;
