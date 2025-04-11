
const moduleRegistry = new Map();
const modulesCreated = new Map();
moduleRegistry.set("vs/nls", {
  exports: {
    create: (x, y) => console.log("CREATE", x, y),
  },
});
moduleRegistry.set("vs/nls!vs/editor/common/languages", {
  exports: {
    localized: (x, y) => console.log("LOCALIZE", x, y),
  },
});

const factoryRegistry = new Map();

const requireFunc = (name) => moduleRegistry.get(name);
const define = (name, dependencyNames, factory) => {

  factoryRegistry.set(name, {
    factory: factory,
    dependencyNames: dependencyNames,
  })

  // factory(...deps);

  // console.log(`Module ${name} factory registered:`, module.exports);
};

const createModule = (name) => {
  const util = require("util");
  // console.log(`Defining module: ${name}`);

  const fd = factoryRegistry.get(name);
  if (!fd) {
    console.log("FD not found", name);
  }
  const dependencyNames = fd.dependencyNames;
  const factory = fd.factory;

  const module = {
    exports: {}
  };

  moduleRegistry.set(name, module);

  // Recursively create the deps
  for (const depName of dependencyNames) {
    if (!modulesCreated.has(depName) && depName != "require" && depName != "exports") {
      createModule(depName);
      modulesCreated.set(depName, true);
    }
  }

  const deps = [];

  // Call factory with dependencies and module.exports
  // console.log("DependencyNames", dependencyNames);
  // console.log("Dependencies", util.inspect(deps, {depth: null}));

  for (const dep of dependencyNames) {
    if (dep == "require") {
      deps.push(requireFunc);
    } else if (dep == "exports") {
      deps.push(module.exports);
    } else if (moduleRegistry.has(dep)) {
      // console.log("Using", dep, "with contents", moduleRegistry.get(dep));
      deps.push(moduleRegistry.get(dep).exports);
    } else {
      // console.log("New", dep);
      const newDep = {
        exports: {},
      }
      deps.push(newDep.exports);
      moduleRegistry.set(dep, newDep);
    }
  }

  factory(...deps);
};


