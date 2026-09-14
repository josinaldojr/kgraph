# python-extraction

## Purpose

Extrai um grafo de conhecimento de código de repositórios Python, incluindo modules, packages, classes, functions, imports, herança, chamadas, decorators (Flask, FastAPI, SQLAlchemy), e dependências.

## Behavior

### Entities Extracted

| Entidade Python | NodeType kgraph | ID Pattern |
|---|---|---|
| package (dir com __init__.py) | `Package` | `pkg:mypackage` |
| module (arquivo .py) | `Package` | `pkg:mypackage.users` |
| class | `Struct` | `mypackage.users.UserService` |
| dataclass | `Struct` (com property `dataclass=true`) | `mypackage.models.User` |
| ABC/Protocol | `Interface` | `mypackage.protocols.Repository` |
| enum | `Enum` | `mypackage.models.Status` |
| function | `Function` | `mypackage.users.format_user()` |
| method | `Function` | `mypackage.users.UserService.save()` |
| static method | `Function` (com property `static=true`) | `mypackage.utils.Helper.format()` |
| class method | `Function` (com property `classmethod=true`) | `mypackage.models.User.from_dict()` |
| property | `Field` | `mypackage.users.UserService#name` |
| class variable | `Field` (com property `classvar=true`) | `mypackage.users.UserService#MAX_SIZE` |

### Relationships Extracted

| Relacionamento | EdgeType | Detecção |
|---|---|---|
| import | `imports` | `import mypackage.foo` |
| from import | `imports` | `from mypackage.foo import Bar` |
| class extends | `extends` | `class Foo(Bar):` |
| class implements (ABC) | `implements` | `class Foo(ABC):` |
| class implements (Protocol) | `implements` | `class Foo(Protocol):` |
| function/method call | `calls` | `self.save()`, `foo()` |
| method belongs to class | `has_method` | Método dentro de classe |
| @app.route() | `routed` | Decorator em função |
| @app.get()/@app.post() | `routed` | Decorator em função (FastAPI) |
| @dataclass | `decorated` | Decorator em classe |
| @Entity | `maps_to_table` | Decorator em classe (SQLAlchemy) |
| @Column | `has_field` | Decorator em atributo |

### Flask Route Extraction

```
@app.route('/users', methods=['GET'])
def get_users():
    ...

@app.route('/users/<int:user_id>', methods=['GET'])
def get_user(user_id):
    ...

→
  Node: mypackage.app.get_users() (Function)
  Node: endpoint:GET /users (Endpoint)
  Edge: ...get_users() --routed--> endpoint:GET /users

  Node: mypackage.app.get_user() (Function)
  Node: endpoint:GET /users/<int:user_id> (Endpoint)
  Edge: ...get_user() --routed--> endpoint:GET /users/<int:user_id>
```

### FastAPI Route Extraction

```
@app.get("/users/{user_id}")
async def get_user(user_id: int):
    ...

@app.post("/users")
async def create_user(user: UserCreate):
    ...

→
  Node: mypackage.app.get_user() (Function)
  Node: endpoint:GET /users/{user_id} (Endpoint)
  Edge: ...get_user() --routed--> endpoint:GET /users/{user_id}

  Node: mypackage.app.create_user() (Function)
  Node: endpoint:POST /users (Endpoint)
  Edge: ...create_user() --routed--> endpoint:POST /users
```

### SQLAlchemy Model Extraction

```
class User(Base):
    __tablename__ = 'users'

    id = Column(Integer, primary_key=True)
    name = Column(String(100), nullable=False)
    orders = relationship("Order", back_populates="user")

→
  Node: mypackage.models.User (Struct)
    property: table = users

  Edge: mypackage.models.User --maps_to_table--> table:users

  Node: mypackage.models.User#id (Field)
    property: column = id
    property: type = Integer

  Node: mypackage.models.User#name (Field)
    property: column = name
    property: type = String(100)
```

### Dependency Extraction

Lê `requirements.txt`, `pyproject.toml` ou `setup.py` para extrair dependências.

```
# requirements.txt
flask>=2.0.0
sqlalchemy>=1.4.0

→
  Node: ext:flask (ExternalDependency)
    property: version = >=2.0.0

  Node: ext:sqlalchemy (ExternalDependency)
    property: version = >=1.4.0
```

### Call Resolution

Call resolution é best-effort:
- `self.method()` → resolve para método na mesma classe
- `cls.method()` → resolve para classmethod
- `module.function()` → resolve para função em módulo importado
- `function()` → resolve para função importada ou local

## Edge Cases

- Decorators empilhados: todos extraídos
- Funções anônimas (lambda): ignoradas (não têm nome estático)
- List comprehensions: ignoradas (não são declarações)
- Type hints: extraídos como properties mas não resolvidos
- `__init__.py`: tratado como module node
- Relative imports: resolvidos relativamente ao package

## Acceptance Criteria

- [ ] Extrai packages e modules corretamente
- [ ] Extrai classes, dataclasses, ABCs, enums
- [ ] Extrai functions (incluindo static methods, class methods)
- [ ] Extrai imports (import e from...import)
- [ ] Extrai herança (extends) e implementação (ABC/Protocol)
- [ ] Extrai chamadas (best-effort)
- [ ] Extrai decorators Flask (@app.route)
- [ ] Extrai decorators FastAPI (@app.get, @app.post)
- [ ] Extrai decorators SQLAlchemy (@Entity, @Column)
- [ ] Extrai dependências (requirements.txt, pyproject.toml)
- [ ] IDs são determinísticos e idempotentes
- [ ] Testes com fixture FastAPI completo
