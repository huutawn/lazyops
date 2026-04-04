import nextCoreWebVitals from 'eslint-config-next/core-web-vitals';

const eslintConfig = [
  {
    ignores: ['.next/**', 'coverage/**', 'node_modules/**'],
  },
  ...nextCoreWebVitals,
];

export default eslintConfig;
