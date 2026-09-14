# java-extraction

## Purpose

Extrai um grafo de conhecimento de código de repositórios Java, incluindo classes, interfaces, enums, métodos, campos, imports, herança, chamadas, annotations Spring/JPA, e dependências Maven/Gradle.

## Behavior

### Entities Extracted

| Entidade Java | NodeType kgraph | ID Pattern |
|---|---|---|
| package | `Package` | `pkg:com.example.service` |
| class | `Struct` | `com.example.service.UserService` |
| abstract class | `Struct` (com property `abstract=true`) | `com.example.service.BaseController` |
| interface | `Interface` | `com.example.service.UserRepository` |
| enum | `Enum` | `com.example.model.Status` |
| record (Java 16+) | `Struct` (com property `record=true`) | `com.example.dto.UserDTO` |
| method | `Function` | `com.example.service.UserService.save()` |
| static method | `Function` (com property `static=true`) | `com.example.util.Helper.format()` |
| constructor | `Function` (com property `constructor=true`) | `com.example.service.UserService.<init>()` |
| field | `Field` | `com.example.service.UserService#name` |
| static field | `Field` (com property `static=true`) | `com.example.util.Constants#MAX_SIZE` |

### Relationships Extracted

| Relacionamento | EdgeType | Detecção |
|---|---|---|
| import | `imports` | `import com.example...` |
| class extends | `extends` | `class Foo extends Bar` |
| interface extends | `extends` | `interface Foo extends Bar` |
| class implements | `implements` | `class Foo implements Bar` |
| method call | `calls` | `this.save()`, `userService.save()` |
| field access | `has_field` | Declaração de campo |
| method belongs to class | `has_method` | Método dentro de classe |
| @Autowired/@Inject | `injected` | Annotation em campo/construtor |
| @Service/@Component/@Repository | `decorated` | Annotation em classe |
| @RestController/@Controller | `decorated` | Annotation em classe |
| @GetMapping/@PostMapping/etc | `routed` | Annotation em método |
| @Entity | `maps_to_table` | Annotation em classe |
| @Table | `maps_to_table` | Annotation em classe (nome da tabela) |
| @Column | `has_field` | Annotation em campo (nome da coluna) |

### Spring/JPA Annotation Extraction

```
@Entity
@Table(name = "users")
public class User {
    @Id
    @GeneratedValue
    private Long id;

    @Column(name = "user_name")
    private String name;

    @OneToMany(mappedBy = "user")
    private List<Order> orders;
}

→
  Node: com.example.model.User (Struct)
    property: annotation = @Entity
    property: table = users

  Node: table:users (Table)

  Edge: com.example.model.User --maps_to_table--> table:users

  Node: com.example.model.User#id (Field)
    property: column = id
    property: generated = true

  Node: table:users.id (Column)

  Edge: table:users --has_field--> table:users.id
```

### REST Controller Extraction

```
@RestController
@RequestMapping("/api/users")
public class UserController {
    @GetMapping("/{id}")
    public User getUser(@PathVariable Long id) { ... }

    @PostMapping
    public User createUser(@RequestBody User user) { ... }
}

→
  Node: com.example.controller.UserController (Struct)
    property: annotation = @RestController
    property: base_path = /api/users

  Node: com.example.controller.UserController.getUser() (Function)

  Node: endpoint:GET /api/users/{id} (Endpoint)

  Edge: com.example.controller.UserController.getUser() --routed--> endpoint:GET /api/users/{id}

  Node: com.example.controller.UserController.createUser() (Function)

  Node: endpoint:POST /api/users (Endpoint)

  Edge: com.example.controller.UserController.createUser() --routed--> endpoint:POST /api/users
```

### Dependency Injection Extraction

```
@Service
public class UserService {
    @Autowired
    private UserRepository userRepository;

    @Inject
    private EmailService emailService;
}

→
  Node: com.example.service.UserService (Struct)
    property: annotation = @Service

  Edge: com.example.service.UserService --injected--> com.example.service.UserRepository
  Edge: com.example.service.UserService --injected--> com.example.service.EmailService
```

### Maven/Gradle Dependency Extraction

Lê `pom.xml` ou `build.gradle`/`build.gradle.kts` para extrair dependências como `ExternalDependency` nodes.

```xml
<!-- pom.xml -->
<dependency>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-web</artifactId>
</dependency>

→
  Node: ext:org.springframework.boot:spring-boot-starter-web (ExternalDependency)
    property: group_id = org.springframework.boot
    property: artifact_id = spring-boot-starter-web
```

### Call Resolution

Call resolution é best-effort, sem type-checking completo:
- `this.method()` ou `method()` → resolve para método na mesma classe
- `variable.method()` → resolve para método com mesmo nome em classes importadas
- `StaticClass.method()` → resolve para método estático
- Chamadas polimórficas via interface → não resolvidas (skip silencioso)

## Edge Cases

- Classes anônimas: extraídas como Struct com nome gerado (`AnonymousClass$1`)
- Classes internas: extraídas com separador `$` (`OuterClass.InnerClass`)
- Genéricos: ignorados na extração de tipo (type erasure)
- Annotations customizadas: extraídas como `Decorator` nodes
- Lombok (@Data, @Getter, @Setter): extraídos como properties, não geram métodos
- Múltiplos annotations na mesma classe: todos extraídos

## Acceptance Criteria

- [ ] Extrai packages Java corretamente
- [ ] Extrai classes, interfaces, enums, records
- [ ] Extrai métodos (incluindo construtores e métodos estáticos)
- [ ] Extrai campos (incluindo campos estáticos)
- [ ] Extrai imports e cria arestas `imports`
- [ ] Extrai herança (`extends`) e implementação (`implements`)
- [ ] Extrai chamadas de método (best-effort)
- [ ] Extrai annotations Spring (@Service, @RestController, @Autowired, @GetMapping, etc.)
- [ ] Extrai annotations JPA (@Entity, @Table, @Column) e cria Table/Column nodes
- [ ] Extrai dependências Maven (pom.xml)
- [ ] Extrai dependências Gradle (build.gradle)
- [ ] IDs são determinísticos e idempotentes
- [ ] Warnings são coletados para erros de parsing (não-fatal)
- [ ] Testes com fixture Spring Boot completo
