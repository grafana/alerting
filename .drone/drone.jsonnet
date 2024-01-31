local image = 'golang:1.21.6';

local pipeline = {
  new(name):: {
    kind: 'pipeline',
    name: name,
  },
};

local step = {
  make(target):: {
    name: 'make-%s' % target,
    image: image,
    commands: ['make %s' % target],
  },
};

[
  pipeline.new('validate-pull-request') {
    steps: [
      step.make('mod-check'),
      step.make('lint'),
      step.make('test'),
    ],
  },
]
