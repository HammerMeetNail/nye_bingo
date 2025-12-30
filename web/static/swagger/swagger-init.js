window.addEventListener('load', () => {
  window.ui = SwaggerUIBundle({
    url: '/static/openapi.yaml',
    dom_id: '#swagger-ui',
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    layout: 'StandaloneLayout',
  });
});
