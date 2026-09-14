# typescript-extraction

## Purpose

Extrai um grafo de conhecimento de código de repositórios TypeScript e JavaScript, incluindo modules, classes, interfaces, types, functions, imports, herança, chamadas, decorators (NestJS, TypeORM), e dependências package.json.

## Behavior

### Entities Extracted

| Entidade TS/JS | NodeType kgraph | ID Pattern |
|---|---|---|
| module (arquivo) | `Package` | `pkg:@myapp/users` |
| class | `Struct` | `@myapp/users.UserService` |
| abstract class | `Struct` (com property `abstract=true`) | `@myapp/users.BaseController` |
| interface | `Interface` | `@myapp/users.User` |
| type alias | `TypeAlias` | `@myapp/users.Status` |
| enum | `Enum` | `@myapp/users.Role` |
| function | `Function` | `@myapp/users.formatUser()` |
| method | `Function` | `@myapp/users.UserService.save()` |
| arrow function (const) | `Function` | `@myapp/users.formatUser()` |
| property | `Field` | `@myapp/users.UserService#name` |

### Relationships Extracted

| Relacionamento | EdgeType | Detecção |
|---|---|---|
| ES import | `imports` | `import { Foo } from './foo'` |
| CommonJS require | `imports` | `const foo = require('./foo')` |
| class extends | `extends` | `class Foo extends Bar` |
| interface extends | `extends` | `interface Foo extends Bar` |
| class implements | `implements` | `class Foo implements Bar` |
| function/method call | `calls` | `this.save()`, `foo()` |
| method belongs to class | `has_method` | Método dentro de classe |
| @Injectable() | `decorated` | Decorator em classe |
| @Controller() | `decorated` | Decorator em classe |
| @Get()/@Post() | `routed` | Decorator em método |
| @Entity() | `maps_to_table` | Decorator em classe |
| @Column() | `has_field` | Decorator em propriedade |
| @Inject() | `injected` | Decorator em propriedade |

### NestJS Decorator Extraction

```
@Controller('users')
export class UsersController {
    @Get(':id')
    async getUser(@Param('id') id: string): Promise<User> { ... }

    @Post()
    async createUser(@Body() dto: CreateUserDto): Promise<User> { ... }
}

→
  Node: @myapp/users.UsersController (Struct)
    property: decorator = @Controller
    property: base_path = users

  Node: @myapp/users.UsersController.getUser() (Function)
  Node: endpoint:GET users/:id (Endpoint)
  Edge: ...getUser() --routed--> endpoint:GET users/:id

  Node: @myapp/users.UsersController.createUser() (Function)
  Node: endpoint:POST users (Endpoint)
  Edge: ...createUser() --routed--> endpoint:POST users
```

### Package.json Dependency Extraction

Lê `package.json` para extrair dependências como `ExternalDependency` nodes.

## Edge Cases

- Arrow functions atribuídas a const: extraídas como Function
- Destructuring imports: múltiplos nós imports
- Dynamic imports: ignorado (não estático)
- Re-exports: import + re-export
- Generics: ignorados na extração de tipo
- Decorators customizados: extraídos como Decorator nodes

## Acceptance Criteria

- [ ] Extrai modules, classes, interfaces, enums, type aliases
- [ ] Extrai functions (incluindo arrow functions e métodos)
- [ ] Extrai imports (ES modules e CommonJS)
- [ ] Extrai herança (extends) e implementação (implements)
- [ ] Extrai chamadas (best-effort)
- [ ] Extrai decorators NestJS (@Controller, @Get, @Post, @Injectable)
- [ ] Extrai decorators TypeORM (@Entity, @Column)
- [ ] Extrai dependências package.json
- [ ] IDs são determinísticos e idempotentes
- [ ] Testes com fixture NestJS completo
