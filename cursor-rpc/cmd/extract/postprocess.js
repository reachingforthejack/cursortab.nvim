  const typeRegistry = new Map();
  const enumRegistry = new Map();
  const serviceRegistry = new Map();

  const getScalarName = (scalar) => {
    switch (scalar) {
      case 1:
        return "double";
      case 2:
        return "float";
      case 3:
        return "int64";
      case 4:
        return "uint64";
      case 5:
        return "int32";
      case 6:
        return "fixed64";
      case 7:
        return "fixed32";
      case 8:
        return "bool";
      case 9:
        return "string";
      case 12:
        return "bytes";
      case 13:
        return "uint32";
      case 15:
        return "sfixed32";
      case 16:
        return "sfixed64";
      case 17:
        return "sint32";
      case 18:
        return "sint64";
      default:
        console.error("No type for scalar", scalar);
    }
    // (s[(s.DOUBLE = 1)] = "DOUBLE"),
    // (s[(s.FLOAT = 2)] = "FLOAT"),
    // (s[(s.INT64 = 3)] = "INT64"),
    // (s[(s.UINT64 = 4)] = "UINT64"),
    // (s[(s.INT32 = 5)] = "INT32"),
    // (s[(s.FIXED64 = 6)] = "FIXED64"),
    // (s[(s.FIXED32 = 7)] = "FIXED32"),
    // (s[(s.BOOL = 8)] = "BOOL"),
    // (s[(s.STRING = 9)] = "STRING"),
    // (s[(s.BYTES = 12)] = "BYTES"),
    // (s[(s.UINT32 = 13)] = "UINT32"),
    // (s[(s.SFIXED32 = 15)] = "SFIXED32"),
    // (s[(s.SFIXED64 = 16)] = "SFIXED64"),
    // (s[(s.SINT32 = 17)] = "SINT32"),
    // (s[(s.SINT64 = 18)] = "SINT64");
  };

  const registerEnum = (prefix, enumDef, registerGlobal = true) => {
    if (enumRegistry.has(enumDef.typeName)) {
      return enumRegistry.get(enumDef.typeName);
    }

    const nameSpl = enumDef.typeName.replace(prefix, "").split(".");
    const name = nameSpl[nameSpl.length - 1];
    // console.log("Replaced", enumDef.typeName, "with", name, "using prefix", prefix);

    const e = {
      name: name,
      fields: enumDef.values.map((e, i) => `${e.name} = ${i};`),
    };

    e.lines = [`enum ${name} { // ${enumDef.typeName}`];
    e.lines.push(...(e.fields.map((f) => `\t${f}`)));
    e.lines.push("}");

    if (registerGlobal) {
      enumRegistry.set(enumDef.typeName, e);
    }

    // console.log("enum", e);

    return e;
  };

  const registerTypeDefinition = (prefix, typeDef, registerGlobal = true) => {
    const nameSpl = typeDef.typeName.replace(prefix, "").split(".");
    const name = nameSpl[nameSpl.length - 1];

    const fields = [];

    const typeObj = {
      name: name,
      locals: new Map(),
    };

    if (registerGlobal) {
      typeRegistry.set(typeDef.typeName, typeObj);
    }

    let preamble = [];

    for (const field of typeDef.fields._fields()) {
      let fieldType = null;
      const repeated = field["repeated"];
      const opt = field["opt"];

      const fieldPrefix = repeated ? "repeated " : opt ? "optional " : "";

      if (field.kind == "message") {
        if (field.T.typeName == typeDef.typeName) {
          fieldType = name;
        } else if (!typeRegistry.has(field.T.typeName)) {
          let registerGlobal = true;
          const newFieldName = field.T.typeName.replace(prefix, "");
          if (newFieldName.split(".").length > 1) {
            registerGlobal = false;
          }

          // if (name == "FileDiff") {
          //   console.log("Full name", typeDef.typeName);
          //   console.log("Should be global", registerGlobal, newFieldName);
          //   console.log("\t", field.T.typeName, "PREFIX", prefix);
          // }

          // const newPrefix = prefix + name + ".";
          // console.log("Current type", prefix, name);
          const newFieldDef = registerTypeDefinition(prefix, field.T, registerGlobal);
          if (!registerGlobal) {
            if (!typeObj.locals.has(field.T.typeName)) {
              typeObj.locals.set(field.T.typeName, true);
              preamble.push(...(newFieldDef.lines.map((l) => `\t${l}`)));
            }
          }
          fieldType = newFieldDef.name;
        } else {
          fieldType = typeRegistry.get(field.T.typeName).name;
        }
      } else if (field.kind == "scalar") {
        fieldType = getScalarName(field.T);
      } else if (field.kind == "enum") {
        const enumName = field.T.typeName.replace(prefix, "");
        let registerGlobal = true;
        if (enumName.split(".").length > 1) {
          registerGlobal = false;
        }
        const newEnum = registerEnum(prefix, field.T, registerGlobal);
        if (!registerGlobal) {
          if (!typeObj.locals.has(field.T.typeName)) {
            typeObj.locals.set(field.T.typeName, newEnum);
            preamble.push(...(newEnum.lines.map((l) => `\t${l}`)));
          }
        }
        fieldType = newEnum.name;
      } else {
        console.error("idk type", field);
        continue;
      }

      fields.push(`${fieldPrefix}${fieldType} ${field.name} = ${field.no};`);
    }

    typeObj.fields = fields;
    typeObj.lines = [
      `message ${name} { // ${typeDef.typeName}`,
    ];
    typeObj.lines.push(...(preamble.map((pl) => `\t${pl}`)));
    typeObj.lines.push(...(fields.map((f) => `\t${f}`)));
    typeObj.lines.push("}");

    // console.log("Type", typeObj);

    return typeObj;
  };

  const registerService = (s) => {
    // console.log("Registering", f);
    const tn = s.typeName;
    const spl = tn.split(".");
    const serviceName = spl[spl.length - 1];
    const packageName = spl.slice(0, spl.length - 1).join(".");

    const lines = [
      "service " + serviceName + " {",
    ];

    for (const [_, method] of Object.entries(s.methods)) {
      const inType = registerTypeDefinition(packageName + ".", method.I);
      const outType = registerTypeDefinition(packageName + ".", method.O);
      const streaming = method.kind == 1;
      // console.log("method", method.name, "In", inType.name, "Out", outType.name);
      lines.push(`\trpc ${method.name}(${inType.name}) returns (${streaming ? "stream" : ""} ${outType.name}) {}`)
    }

    lines.push("}");

    const service = {
      package: packageName,
      name: serviceName,
      lines: lines,
    };

    serviceRegistry.set(tn, service);
    // console.log("Service", service);
  }

  const modulesToGenerate = [
    {
      name:"aiserver",
    },
    {
      name: "repository",
    },
  ].map((module) => ({
    ...module,
    modulePath: `proto/aiserver/v1/${module.name}_connectweb`,
  }));

  for (const mtg of modulesToGenerate) {
    createModule(mtg.modulePath);
    const mod = moduleRegistry.get(mtg.modulePath);
    // This seems to work for now /shrug
    const exportName =Object.keys(mod.exports)[0];
    mtg.exports = mod.exports[exportName];
    registerService(mtg.exports);

    // mtg.text = protoContent;

    // enumRegistry.clear();
    // typeRegistry.clear();
    // serviceRegistry.clear();
  }

  // const aiserver = moduleRegistry.get(toGenerate).exports["$w5"];
  // const reposerver = moduleRegistry.get("proto/aiserver/v1/repository_connectweb").exports["$I7"];

  // const streamChat = aiserver.methods.streamChat;
  // const inType = registerTypeDefinition("aiserver.v1.", streamChat.I);
  // const outType = registerTypeDefinition("aiserver.v1.", streamChat.O);
  // console.log("SERVICE", aiserver);
  // registerService(aiserver);
  // registerService(reposerver);

  // console.log("METHOD", streamChat);
  // enumRegistry.forEach((e) => console.log(e.lines.join("\n")));
  // typeRegistry.forEach((t) => console.log(t.lines.join("\n")));
  // serviceRegistry.forEach((s) => console.log(s.lines.join("\n")));


  // Naively assume the last arg is a path to an output directory
  const outPath = process.argv[process.argv.length - 1];

  // We need a separate file for each service we want to save
  const fs = require("fs");
  const path = require("path");

  if (!fs.existsSync(outPath)) {
    fs.mkdirSync(outPath, { recursive: true });
  }


  let protoContent = 'syntax = "proto3";\n';
  protoContent += `package aiserver.v1;\n`;
  protoContent += `option go_package = "cursor/gen/aiserver/v1;aiserverv1";\n`;

  for (const [_, e] of enumRegistry) {
    protoContent += e.lines.join("\n");
    protoContent += "\n";
  }

  for (const [_, t] of typeRegistry) {
    protoContent += t.lines.join("\n");
    protoContent += "\n";
  }

  for (const [_, s] of serviceRegistry) {
    protoContent += s.lines.join("\n");
    protoContent += "\n";
  }


  const dirPath = path.join(outPath, "aiserver", "v1");
  fs.mkdirSync(dirPath, { recursive: true });
  const protoFilePath = path.join(dirPath, `aiserver.proto`);

  // for (const mtg of modulesToGenerate) {
  //   protoContent += mtg.text;
  // }
  fs.writeFileSync(protoFilePath, protoContent);

