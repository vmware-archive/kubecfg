function(input)
  assert input.top;
  {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: {
      name: 'sink',
    },
    data: {
      input: input,
      var: std.extVar('filevar'),
    },
  }
